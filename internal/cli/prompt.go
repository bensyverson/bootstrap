package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// terminal is the person at the other end of bootstrap new: the flow.Person that
// asks at a keyboard, or takes each default when no one is there to ask. It also
// prints the flow's work from the log as it happens, in wait's rows.
type terminal struct {
	mu        sync.Mutex
	out       io.Writer
	store     session.Store
	answering answering
	// quiet prints nothing, for --json.
	quiet bool
	lines <-chan string
	// told is the last log seq printed, and updates everything printed this run.
	told    int
	updates []session.Update
}

// answering is who answers the flow's questions in a run of new.
type answering int

const (
	// refuseQuestions: no one is at a keyboard and there's no --yes, so any
	// question is refused, never defaulted (Ben, 2026-11-05, OWv6U2).
	refuseQuestions answering = iota
	// takeDefaults: --yes, so each question takes its default.
	takeDefaults
	// askPerson: a person at a keyboard answers.
	askPerson
)

func newTerminal(c *call, answering answering, quiet bool) *terminal {
	t := &terminal{out: c.env.Stdout, answering: answering, quiet: quiet}
	if answering == askPerson {
		t.lines = readLines(c.env.Stdin)
	}
	return t
}

// attach starts printing the flow's work from a store's log. An earlier run's work was told then.
func (t *terminal) attach(store session.Store) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.store = store
	if state, err := store.Load(); err == nil {
		t.told = state.Seq
	}
}

// readLines hands over stdin a line at a time, so a question can give up when
// the run is interrupted. The channel closes at the end of input.
func readLines(in io.Reader) <-chan string {
	lines := make(chan string)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(in)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	return lines
}

// unanswered is a question nobody answered: no one was there to ask and it has
// no default, or input ended first.
type unanswered struct {
	q flow.Question
}

func (u *unanswered) Error() string { return "nothing answers “" + spans(u.q.Screen.Title) + "”" }

// watch prints the flow's work each time the log changes, until ctx ends.
func (t *terminal) watch(ctx context.Context) {
	for {
		rung := t.store.Bell.Rung()
		select {
		case <-ctx.Done():
			return
		case <-rung:
			t.mu.Lock()
			t.flush()
			t.mu.Unlock()
		}
	}
}

// flush prints what the flow has done since the last flush. The caller holds mu.
func (t *terminal) flush() {
	_, updates, err := t.store.Unread()
	if err != nil {
		return
	}
	for _, u := range updates {
		if u.Seq <= t.told {
			continue
		}
		t.told = u.Seq
		t.updates = append(t.updates, u)
		t.print(updateRows(u)...)
	}
}

func (t *terminal) print(rows ...string) {
	if t.quiet {
		return
	}
	for _, row := range rows {
		fmt.Fprintln(t.out, row)
	}
}

// updateRows is one update as wait prints it. A step says only where it ended,
// and the done handoff is left to new's last line.
func updateRows(u session.Update) []string {
	switch {
	case u.Step != nil:
		if u.Step.Status == session.StepDone || u.Step.Status == session.StepSkipped {
			return []string{fmt.Sprintf("step %s %s %d of %d", u.Step.ID, u.Step.Status, u.Step.Index, u.Step.Of)}
		}
	case u.Note != "":
		return []string{"note " + u.Note}
	case u.Handoff != nil && u.Handoff.Kind == session.HandoffTrouble:
		return handoffRows(*u.Handoff)
	}
	return nil
}

func spans(s []screen.Span) string {
	var b strings.Builder
	for _, span := range s {
		b.WriteString(span.Text)
	}
	return b.String()
}

// See prints a screen that asks nothing: one line of progress, or what the person must do, with any code and link.
func (t *terminal) See(desc screen.Description) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.flush()
	switch {
	case desc.Mode == screen.Working && len(desc.Status) > 0:
		t.print(spans(desc.Status))
	case desc.Mode == screen.Waiting:
		t.print(spans(desc.Title))
		if len(desc.Status) > 0 {
			t.print("  " + spans(desc.Status))
		}
		if desc.Code != "" {
			t.print("  code: " + desc.Code)
		}
		for _, c := range desc.Choices {
			if c.URL != "" {
				t.print(fmt.Sprintf("  %s: %s", c.Label, c.URL))
			}
		}
	default:
		t.print(spans(desc.Title))
	}
}

// Ask asks at the keyboard, or takes the default when no one is there.
func (t *terminal) Ask(ctx context.Context, q flow.Question) (screen.Answer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.flush()
	if t.answering != askPerson {
		if t.answering == refuseQuestions || q.Default == nil || q.Refused != nil {
			return screen.Answer{}, &unanswered{q}
		}
		return *q.Default, nil
	}
	if q.Refused != nil {
		t.print("  that didn't work: " + q.Refused.Error())
	} else {
		t.print("")
		t.print(spans(q.Screen.Title))
		if len(q.Screen.Status) > 0 {
			t.print("  " + spans(q.Screen.Status))
		}
	}
	switch {
	case len(q.Screen.Rows) > 0:
		return t.pickTools(ctx, q)
	case q.Screen.Field != nil:
		return t.typeIn(ctx, q)
	}
	return t.choose(ctx, q)
}

// read waits for a line, prompting first.
func (t *terminal) read(ctx context.Context, q flow.Question, prompt string) (string, error) {
	fmt.Fprint(t.out, prompt)
	select {
	case <-ctx.Done():
		fmt.Fprintln(t.out)
		return "", ctx.Err()
	case line, ok := <-t.lines:
		if !ok {
			fmt.Fprintln(t.out)
			return "", &unanswered{q}
		}
		return strings.TrimSpace(line), nil
	}
}

// answerable are a screen's choices that answer it, leaving out links, with the
// main one first. The page lays them out as a Mac does, with the main one on the
// right, but a numbered list reads from the top.
func answerable(desc screen.Description) []screen.Choice {
	choices := slices.DeleteFunc(slices.Clone(desc.Choices), func(c screen.Choice) bool { return c.URL != "" })
	slices.SortStableFunc(choices, func(a, b screen.Choice) int {
		switch {
		case a.Primary == b.Primary:
			return 0
		case a.Primary:
			return -1
		}
		return 1
	})
	return choices
}

func (t *terminal) choose(ctx context.Context, q flow.Question) (screen.Answer, error) {
	choices := answerable(q.Screen)
	for i, c := range choices {
		mark := ""
		if q.Default != nil && q.Default.Choice == c.ID {
			mark = " (default)"
		}
		t.print(fmt.Sprintf("  %d. %s%s", i+1, c.Label, mark))
	}
	for _, c := range q.Screen.Choices {
		if c.URL != "" {
			t.print(fmt.Sprintf("  %s: %s", c.Label, c.URL))
		}
	}
	prompt := fmt.Sprintf("Choose 1-%d: ", len(choices))
	if q.Default != nil {
		i := slices.IndexFunc(choices, func(c screen.Choice) bool { return c.ID == q.Default.Choice })
		prompt = fmt.Sprintf("Choose 1-%d, or press Enter for %s: ", len(choices), choices[i].Label)
	}
	for {
		line, err := t.read(ctx, q, prompt)
		if err != nil {
			return screen.Answer{}, err
		}
		if line == "" && q.Default != nil {
			return *q.Default, nil
		}
		if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(choices) {
			return screen.Answer{Choice: choices[n-1].ID}, nil
		}
		if i := slices.IndexFunc(choices, func(c screen.Choice) bool { return strings.EqualFold(c.ID, line) }); i >= 0 {
			return screen.Answer{Choice: choices[i].ID}, nil
		}
		t.print(fmt.Sprintf("  type a number from 1 to %d", len(choices)))
	}
}

// typeIn asks for a name or an email. Enter takes the default, or else skips when the screen offers that.
func (t *terminal) typeIn(ctx context.Context, q flow.Question) (screen.Answer, error) {
	choices := answerable(q.Screen)
	prompt := q.Screen.Field.Label + ": "
	var skip *screen.Choice
	if len(choices) > 1 {
		skip = &choices[len(choices)-1]
	}
	switch {
	case q.Default != nil && q.Default.Name != "":
		prompt = fmt.Sprintf("%s [%s]: ", q.Screen.Field.Label, q.Default.Name)
	case skip != nil:
		prompt = fmt.Sprintf("%s (Enter skips): ", q.Screen.Field.Label)
	}
	for {
		line, err := t.read(ctx, q, prompt)
		if err != nil {
			return screen.Answer{}, err
		}
		switch {
		case line != "":
			return screen.Answer{Choice: choices[0].ID, Name: line}, nil
		case q.Default != nil:
			return *q.Default, nil
		case skip != nil:
			return screen.Answer{Choice: skip.ID}, nil
		}
	}
}

// pickTools shows the tool list, and takes the names of any to leave out.
func (t *terminal) pickTools(ctx context.Context, q flow.Question) (screen.Answer, error) {
	var optional []string
	for _, row := range q.Screen.Rows {
		suffix := ""
		if row.Required {
			suffix = " (required)"
		} else {
			optional = append(optional, row.ID)
		}
		t.print(fmt.Sprintf("  %-12s %s%s", row.ID, row.Say, suffix))
	}
	keep := screen.AllToolIDs()
	if q.Default != nil {
		keep = q.Default.Tools
	}
	for {
		line, err := t.read(ctx, q, "Type any to leave out, or press Enter to keep them all: ")
		if err != nil {
			return screen.Answer{}, err
		}
		drop := strings.FieldsFunc(line, func(r rune) bool { return r == ' ' || r == ',' })
		if bad := slices.IndexFunc(drop, func(id string) bool { return !slices.Contains(optional, id) }); bad >= 0 {
			t.print(fmt.Sprintf("  %s isn't one you can leave out: %s", drop[bad], strings.Join(optional, ", ")))
			continue
		}
		kept := slices.DeleteFunc(slices.Clone(keep), func(id string) bool { return slices.Contains(drop, id) })
		return screen.Answer{Choice: answerable(q.Screen)[0].ID, Tools: screen.KeepRequired(kept)}, nil
	}
}

func isUnanswered(err error) (*unanswered, bool) {
	var u *unanswered
	ok := errors.As(err, &u)
	return u, ok
}
