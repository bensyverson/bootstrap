// Sleepy probe for scripts/page-fit.sh: opens each preview state in turn and measures how far its words
// stay above the progress percentage, and whether any of them leave the window. Runs in the page's world.
const previews = await (await fetch(withToken("preview.json"))).json();
const wait = ms => new Promise(r => setTimeout(r, ms));
// Headless Sleepy never runs the render loop that writes the percentage, so give it the words it would hold.
const pct = document.getElementById("pct");
pct.textContent = "50% complete";
const pctTop = pct.getBoundingClientRect().top;
// Longer than the page's hold on a working screen.
const SETTLE_MS = 450;
const out = [];
for (const p of previews) {
  if (!p.name.endsWith(only)) continue;
  location.hash = p.name;
  await wait(SETTLE_MS);
  const panel = document.querySelector(".panel:not(.leaving)");
  if (!panel) continue;
  const boxes = [...panel.children].map(c => c.getBoundingClientRect()).filter(r => r.height > 0);
  const bottom = Math.max(...boxes.map(r => r.bottom)), top = Math.min(...boxes.map(r => r.top));
  out.push({ name: p.name, clear: Math.round(pctTop - bottom), top: Math.round(top) });
}
return out;
