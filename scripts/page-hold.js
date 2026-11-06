// Sleepy probe for scripts/page-hold.sh: walks the preview page between screens on a clock and records
// every panel that draws. Runs as an async function body in the page's world, where withToken is global.
const previews = await (await fetch(withToken("preview.json"))).json();
const frameOf = name => previews.find(p => p.name === name).frame;
const titleOf = name => frameOf(name).screen.title.map(s => s.text).join("");
const statusOf = name => (frameOf(name).screen.status || []).map(s => s.text).join("");
const wait = ms => new Promise(r => setTimeout(r, ms));
const go = name => { location.hash = name; };

let drawn = [];
new MutationObserver(changes => changes.forEach(c => c.addedNodes.forEach(n => {
  if (n.classList && n.classList.contains("panel")) drawn.push(n.querySelector("h1").textContent);
}))).observe(document.getElementById("words"), { childList: true });
const status = () => { const s = document.querySelector(".panel:not(.leaving) .statusBox:not(.gone)"); return s ? s.textContent : ""; };

// Long enough for any hold to run out, so each case starts from a screen at rest.
const SETTLE_MS = 900, QUICK_MS = 40;
async function from(name) { go(name); await wait(SETTLE_MS); drawn = []; }

const cases = {};
const AGENT = ".claude-code";

// The trial's two flashes: a working screen replaced before the hold runs out never draws.
await from("list.answered" + AGENT);
go("fast" + AGENT); await wait(QUICK_MS); go("apple-ask" + AGENT); await wait(SETTLE_MS);
cases.fastBeforeAppleNeverDraws = !drawn.includes(titleOf("fast" + AGENT)) && drawn.includes(titleOf("apple-ask" + AGENT));

await from("naming.answered" + AGENT);
go("building.folder" + AGENT); await wait(QUICK_MS); go("git-ask" + AGENT); await wait(SETTLE_MS);
cases.buildingBeforeGitNeverDraws = !drawn.includes(titleOf("building.folder" + AGENT)) && drawn.includes(titleOf("git-ask" + AGENT));

// A newer working screen replaces a held one, and is held in its turn.
await from("list.answered" + AGENT);
go("fast" + AGENT); await wait(QUICK_MS); go("apple-wait" + AGENT); await wait(SETTLE_MS);
cases.newerWorkingScreenWins = !drawn.includes(titleOf("fast" + AGENT)) && drawn.includes(titleOf("apple-wait" + AGENT));

// A slow step still shows its working screen.
await from("list.answered" + AGENT);
go("fast" + AGENT); await wait(SETTLE_MS);
cases.slowStepShowsItsScreen = drawn.includes(titleOf("fast" + AGENT));

// A screen that isn't working draws at once.
await from("welcome" + AGENT);
go("git-ask" + AGENT); await wait(QUICK_MS);
cases.questionDrawsAtOnce = drawn.includes(titleOf("git-ask" + AGENT));

// A new status under the working screen already shown changes at once, so steady updates never starve it.
await from("tool.agents.downloading" + AGENT);
go("tool.agents.installing" + AGENT); await wait(QUICK_MS);
cases.statusUnderShownScreenAtOnce = status() === statusOf("tool.agents.installing" + AGENT);

return { ok: Object.values(cases).every(Boolean), cases };
