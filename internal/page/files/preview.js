/* The preview route: one screen in one state from its address, at rest, for review and still tests.
   After that, a new address or the arrow keys walk the screens with the real transitions. */
const state = () => decodeURIComponent(location.hash.slice(1));

(async () => {
  const previews = await (await fetch(withToken("preview.json"))).json();
  await ready;
  const find = name => previews.find(p => p.name === name);

  const first = find(state());
  atRest = !!first;
  if (first) draw(first.frame);
  else listAll(previews);
  startThreads();
  document.documentElement.dataset.drawn = "true";

  addEventListener("hashchange", () => {
    const next = find(state());
    atRest = false;
    if (next) draw(next.frame);
  });
  addEventListener("keydown", e => {
    const step = { ArrowRight: 1, ArrowLeft: -1 }[e.key];
    if (!step || e.target.matches("input")) return;
    const at = previews.findIndex(p => p.name === state());
    location.hash = previews[(at + step + previews.length) % previews.length].name;
  });
})();

function listAll(previews) {
  const list = el("nav", "previews");
  list.append(...previews.map(p => {
    const a = el("a", "", p.name);
    a.href = withToken("preview") + "#" + p.name;
    a.onclick = () => { location.href = a.href; location.reload(); return false; };
    return a;
  }));
  $("words").append(list);
}
