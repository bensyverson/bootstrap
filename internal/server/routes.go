package server

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/bensyverson/bootstrap/internal/page"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

const (
	maxAnswerBytes = 64 * 1024
	keepAlive      = 15 * time.Second
)

// answerRequest is what the page posts when a person decides.
type answerRequest struct {
	Seq    int      `json:"seq"`
	Choice string   `json:"choice"`
	Tools  []string `json:"tools"`
	Name   string   `json:"name"`
	In     string   `json:"in"`
}

// answerReply tells the page what became of the click.
type answerReply struct {
	Saved bool   `json:"saved"`
	Lost  bool   `json:"lost,omitempty"`
	Error string `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, reply answerReply) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(reply)
}

// guard refuses anything that isn't this page, on this address, holding this token.
// Checking Host stops DNS rebinding, and Origin stops another site posting to us.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", s.policy)
		origin := r.Header.Get("Origin")
		token := r.URL.Query().Get("token")
		var refusal string
		switch {
		case r.Host != s.host:
			refusal = "wrong host"
		case subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.Token)) != 1:
			refusal = "wrong token"
		case origin != "" && origin != s.origin:
			refusal = "wrong origin"
		case origin == "" && r.Method != http.MethodGet && r.Method != http.MethodHead:
			refusal = "no origin"
		}
		if refusal != "" {
			writeJSON(w, http.StatusForbidden, answerReply{Error: refusal})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", s.handleEvents)
	mux.HandleFunc("/answer", s.handleAnswer)
	mux.HandleFunc("/preview.json", s.handlePreviews)
	mux.HandleFunc("/", s.handlePage)
	return mux
}

// handlePreviews lists what the preview route can draw, which is the page at
// /preview. It never touches the session, so a preview can't stand in for a tab.
func (s *Server) handlePreviews(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(page.Previews())
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" || name == "preview" {
		name = "index.html"
	}
	if !fs.ValidPath(name) {
		http.NotFound(w, r)
		return
	}
	body, err := fs.ReadFile(s.cfg.Page, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
}

func (s *Server) handleAnswer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, answerReply{Error: "answers are posted"})
		return
	}
	// Plain forms can be sent across sites without a preflight, and JSON can't.
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, answerReply{Error: "answers are JSON"})
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAnswerBytes))
	decoder.DisallowUnknownFields()
	var req answerRequest
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, answerReply{Error: "that isn't an answer"})
		return
	}
	now := s.cfg.Now()
	_, err := s.cfg.Store.RecordAnswer(req.Seq, screen.Answer{Choice: req.Choice, Tools: req.Tools, Name: req.Name, In: req.In}, now)
	var invalid *screen.InvalidError
	switch {
	case err == nil:
	case errors.Is(err, session.ErrStale), errors.Is(err, session.ErrAnswered), errors.Is(err, session.ErrEnded):
		writeJSON(w, http.StatusConflict, answerReply{Error: err.Error()})
		return
	case errors.As(err, &invalid):
		writeJSON(w, http.StatusUnprocessableEntity, answerReply{Error: invalid.Message})
		return
	default:
		writeJSON(w, http.StatusInternalServerError, answerReply{Error: "the answer couldn't be saved"})
		return
	}
	// The click may be the moment the page learns the agent has gone.
	s.mu.Lock()
	s.step(now)
	var frame session.Frame
	json.Unmarshal(s.lastFrame, &frame)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, answerReply{Saved: true, Lost: frame.Lost})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	controller := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	client := s.addClient()
	defer s.removeClient(client)
	w.WriteHeader(http.StatusOK)
	controller.Flush()

	alive := time.NewTicker(keepAlive)
	defer alive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case body, open := <-client:
			if !open {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", body); err != nil {
				return
			}
			controller.Flush()
		case <-alive.C:
			if _, err := fmt.Fprint(w, ": still here\n\n"); err != nil {
				return
			}
			controller.Flush()
		}
	}
}

// addClient registers a tab and queues what it should show now.
func (s *Server) addClient() chan []byte {
	client := make(chan []byte, 1)
	s.mu.Lock()
	s.clients[client] = true
	s.everTab = true
	tabs := len(s.clients)
	s.step(s.cfg.Now())
	// publish only sends changes, so a new tab may not have been sent this.
	if len(client) == 0 && s.lastFrame != nil {
		client <- s.lastFrame
	}
	s.mu.Unlock()
	s.cfg.Store.SetTabs(tabs, s.cfg.Now())
	return client
}

func (s *Server) removeClient(client chan []byte) {
	s.mu.Lock()
	_, present := s.clients[client]
	delete(s.clients, client)
	tabs := len(s.clients)
	if tabs == 0 {
		s.emptySince = s.lastStep
	}
	s.mu.Unlock()
	if present {
		s.cfg.Store.SetTabs(tabs, s.cfg.Now())
	}
}
