/* Draws any screen from the description the server sends, and holds no screen's words. */
const WAIT_SPEED = 0.85, WORK_SPEED = 3.8, SETTLED_SPEED = .7;
const calm = matchMedia("(prefers-reduced-motion: reduce)").matches;
const $ = id => document.getElementById(id);
const el = (tag, cls, text) => { const e = document.createElement(tag); if (cls) e.className = cls; if (text !== undefined) e.textContent = text; return e; };
const withToken = file => "/" + file + location.search;

/* A link that opens one screen shows it at rest, with nothing mid-flight. */
let atRest = false;

/* The words here are the page's own, and belong to no screen. */
const COPY_LABEL = "Copy", COPIED_LABEL = "Copied";
// Proposed, not yet approved by Ben: the line a click turns the choices into while the next screen is on its way.
const WAITING_LINE = "One moment…";

/* ---------- Fonts and pictures load before the first screen, so nothing ever arrives half-painted. ---------- */
const FONTS = [["fonts/Grand-Regular.subset.woff2", "400"], ["fonts/Grand-Medium.subset.woff2", "500"]];
const fontsReady = FONTS.map(([file, weight]) => {
  const face = new FontFace("Grand", `url("${withToken(file)}") format("woff2")`, { weight, display: "block" });
  document.fonts.add(face);
  return face.load();
});
const CARDS = {};
const picturesReady = [...document.querySelectorAll("template[data-card]")].flatMap(template => {
  // importNode, because a picture in a template's own document never loads.
  const body = document.importNode(template.content, true);
  const pictures = [...body.querySelectorAll("img[data-src]")];
  pictures.forEach(i => { i.src = withToken(i.dataset.src); });
  CARDS[template.dataset.card] = [...body.childNodes];
  return pictures.map(i => (i.decode ? i.decode() : Promise.resolve()));
});
const ready = Promise.allSettled([...fontsReady, ...picturesReady]);

/* ---------- Transitions: one motion, never two. What leaves and what arrives share a clock. ---------- */
const EASE = "cubic-bezier(.65,0,.35,1)";
const SWAP_MS = 1400, STATUS_MS = 700;
const reach = () => (calm ? 0 : Math.min(110, innerWidth * .08));
let firstPaint = true;
function arrive(node, ms, dx, dy = 0) {
  if (atRest && firstPaint) return;
  node.animate([{ opacity: 0, transform: `translate(${dx}px, ${dy}px)` }, { opacity: 1, transform: "none" }], { duration: ms, easing: EASE, fill: "backwards" });
}
function depart(node, ms, dx, dy = 0) {
  const a = node.animate([{ opacity: 1, transform: "none" }, { opacity: 0, transform: `translate(${dx}px, ${dy}px)` }], { duration: ms, easing: EASE, fill: "forwards" });
  a.onfinish = () => node.remove();
}

/* ---------- Screens ---------- */
let working = false, settled = false, progress = 0, panel = null, shownKey = "", statusSlot = null;
let seq = 0, shown = null, frameAnswered = false, shownCard = "";
/* Sends a person's answer. The live page sets this; a preview leaves it doing nothing. */
let sendAnswer = async () => ({ ok: true });

/* Words are text nodes and links, never HTML, so a frame has nothing to inject. */
function spansInto(parent, spans = []) {
  parent.append(...spans.map(s => {
    if (!s.url) return document.createTextNode(s.text);
    const a = el("a", "", s.text);
    a.href = s.url; a.target = "_blank"; a.rel = "noopener";
    return a;
  }));
  return parent;
}

/* A new working screen waits a moment before it draws: a quick step's next screen often follows within
   milliseconds, and drawing both makes the first one flash. A newer screen replaces a waiting one. Only a
   new screen waits; a status line under the one already shown changes at once, so steady updates never
   hold it back. Until it draws, the screen before it stays, with its seq. */
const HOLD_MS = 300;
let held = null;
function draw(frame) {
  const d = frame.screen;
  const key = d ? screenKey(d) : "";
  if (d && d.mode === "working" && panel && key !== shownKey) {
    if (!held || held.key !== key) {
      clearTimeout(held && held.timer);
      held = { key, timer: setTimeout(() => { const f = held.frame; held = null; paint(f); }, HOLD_MS) };
    }
    held.frame = frame;
    return;
  }
  if (held) { clearTimeout(held.timer); held = null; }
  paint(frame);
}

function paint(frame) {
  const d = frame.screen;
  seq = frame.seq;
  frameAnswered = !!frame.answered;
  if (!d) return;
  // After a click it's the agent's turn, so the object picks up speed at once.
  working = d.mode === "working" || (frameAnswered && !frame.lost);
  settled = d.mode === "settled";
  progress = d.progress;
  if ((d.card || "") !== shownCard) { shownCard = d.card || ""; peek(shownCard); }
  show(d);
  setAnswered(frameAnswered);
  firstPaint = false;
  // Once the state at rest is styled, later changes on this panel ease like any other.
  if (panel && panel.classList.contains("still")) { void panel.offsetWidth; panel.classList.remove("still"); }
}

/* Two descriptions with one key are the same screen, differing at most in their status lines. */
const screenKey = d => JSON.stringify([d.title, (d.choices || []).map(c => c.label), d.rows, d.code, d.field, d.places, d.layout, d.note]);

function show(d) {
  const key = screenKey(d);
  shown = d;
  // A status line changing under the same title moves on its own, and nothing else does.
  if (key === shownKey && panel) { swapStatus(d.status); return; }
  shownKey = key;

  const hadFocus = panel && panel.contains(document.activeElement);
  const old = panel;
  const next = el("section", "panel" + (d.layout === "over" ? " overlay" : "") + (d.layout === "low" ? " low" : "") + (atRest && firstPaint ? " still" : ""));
  next.append(spansInto(el("h1"), d.title));
  if (d.note) next.append(el("p", "note", d.note));
  statusSlot = el("div", "statusSlot");
  if (d.status) statusSlot.append(statusLine(d.status));
  next.append(statusSlot);
  if (d.rows) { const r = el("div", "rows"); r.append(...d.rows.map(rowFor)); next.append(r); }
  if (d.code) {
    const box = el("div", "code"), c = el("code", "", d.code), copy = el("button", "", COPY_LABEL);
    copy.onclick = async () => { try { await navigator.clipboard.writeText(d.code); copy.textContent = COPIED_LABEL; } catch (e) { getSelection().selectAllChildren(c); } };
    box.append(c, copy); next.append(box);
  }
  if (d.field) {
    const f = el("input", "field"); f.value = d.field.value; f.setAttribute("aria-label", d.field.label); f.autocomplete = "off";
    f.placeholder = d.field.label;
    if (d.field.kind === "email") f.type = "email";
    f.oninput = () => swapStatus(shown.status);
    next.append(f);
  }
  if (d.places) next.append(placesFor(d.places));
  const row = el("div", "choices");
  row.append(...(d.choices || []).map(choice => {
    const b = el("button", choice.primary ? "main" : "", choice.label);
    // A button with an address opens it beside the page, and answers nothing.
    b.onclick = () => (choice.url ? open(choice.url, "_blank", "noopener") : answer(choice));
    return b;
  }));
  next.append(row);

  $("words").append(next);
  const R = reach();
  if (old) { old.classList.add("leaving"); depart(old, SWAP_MS, -R); }
  arrive(next, SWAP_MS, R);
  panel = next;
  if (hadFocus && row.firstChild) row.firstChild.focus({ preventScroll: true });
}
/* Where a new project goes: one folder picked of a few, as a row of radio buttons. */
function placesFor(places) {
  const group = el("div", "places");
  group.setAttribute("role", "radiogroup");
  group.setAttribute("aria-label", places.label);
  group.append(el("span", "placesLabel", places.label));
  group.append(...places.options.map(p => {
    const label = el("label", "place"), r = el("input");
    r.type = "radio"; r.name = "place"; r.value = p.path; r.checked = !!p.picked;
    label.append(r, el("span", "", p.path));
    return label;
  }));
  return group;
}
function statusLine(spans) {
  const s = spansInto(el("div", "statusBox"), spans);
  s.setAttribute("role", "status");
  return s;
}
function swapStatus(status) {
  if (!statusSlot) return;
  const old = statusSlot.querySelector(".statusBox:not(.gone)");
  const text = (status || []).map(s => s.text).join("");
  if (old ? old.textContent === text : !text) return;
  const next = statusLine(status);
  statusSlot.append(next);
  // Within a step the line flips upward: the old one rises away as the new one rises in.
  const lift = calm ? 0 : 16;
  if (old) { old.classList.add("gone"); depart(old, STATUS_MS, 0, -lift); }
  arrive(next, STATUS_MS, 0, lift);
}
function rowFor(t) {
  const row = el("label", "row" + (t.required ? " needed" : ""));
  const box = el("input"); box.type = "checkbox"; box.checked = t.checked; box.disabled = t.required;
  box.dataset.id = t.id; box.dataset.required = t.required;
  box.setAttribute("aria-label", t.name);
  const name = spansInto(el("span", "name"), [{ text: t.name, url: t.url }]);
  row.append(box, name, el("span", "say", t.say));
  return row;
}

/* ---------- Answers ---------- */
/* A click answers itself at once: the choices give way to a waiting line in their own place, so nothing
   around them moves, and it holds until the next screen arrives. A refused answer brings them back. */
function setAnswered(on) {
  if (!panel) return;
  panel.classList.toggle("waiting", on);
  panel.querySelectorAll(".choices button").forEach(b => { b.disabled = on; });
  panel.querySelectorAll(".row input").forEach(b => { b.disabled = on || b.dataset.required === "true"; });
  panel.querySelectorAll(".place input").forEach(b => { b.disabled = on; });
  const f = panel.querySelector(".field");
  if (f) f.readOnly = on;
  const row = panel.querySelector(".choices"), line = row && row.querySelector(".waitLine:not(.gone)");
  if (!row || !row.querySelector("button") || on === !!line) return;
  const lift = calm ? 0 : 12;
  if (line) { line.classList.add("gone"); depart(line, SWAP_MS, 0, lift); return; }
  const next = el("div", "waitLine", WAITING_LINE);
  next.setAttribute("role", "status");
  row.append(next);
  arrive(next, SWAP_MS, 0, lift);
}
async function answer(choice) {
  const body = { seq, choice: choice.id };
  if (shown.rows) body.tools = [...panel.querySelectorAll(".row input:checked")].map(b => b.dataset.id);
  if (shown.field) body.name = panel.querySelector(".field").value;
  if (shown.places) body.in = panel.querySelector(".place input:checked").value;
  setAnswered(true);
  let reply = { ok: false };
  try { reply = await sendAnswer(body); } catch (e) { /* The stream notices a server that's gone. */ }
  if (reply.ok) return;
  setAnswered(frameAnswered);
  if (reply.refusal) swapStatus([{ text: reply.refusal }]);
}

/* A card keeps one gentle tilt, and arrives and leaves by its own edge. */
let cardSide = 0;
function peek(name) {
  [...$("peek").children].forEach(c => {
    const out = c.dataset.side === "a" ? -1 : 1, r = c.dataset.tilt;
    const a = c.animate([{ opacity: 1, transform: `translate(0, -50%) rotate(${r}deg)` }, { opacity: 0, transform: `translate(${out * 70}px, -50%) rotate(${r}deg)` }], { duration: 900, easing: EASE, fill: "forwards" });
    a.onfinish = () => c.remove();
  });
  const body = Object.hasOwn(CARDS, name) ? CARDS[name] : null;
  if (!body) return;
  const side = cardSide++ % 2 ? "b" : "a", r = side === "a" ? -1.6 : 1.4, from = side === "a" ? -1 : 1;
  const c = el("div", "card " + side);
  c.dataset.side = side; c.dataset.tilt = r;
  // The same pictures go back in, already decoded. Only a card still on its way out needs a copy.
  c.append(...(body.some(n => n.isConnected) ? body.map(n => n.cloneNode(true)) : body));
  c.style.transform = `translateY(-50%) rotate(${r}deg)`;
  $("peek").append(c);
  if (!atRest) c.animate([{ opacity: 0, transform: `translate(${from * 70}px, -50%) rotate(${r}deg)` }, { opacity: 1, transform: `translate(0, -50%) rotate(${r}deg)` }], { duration: 900, easing: EASE, fill: "backwards" });
}
