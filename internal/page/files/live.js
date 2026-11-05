/* The real page: draws what the server streams, and posts what a person decides. */

// Long enough that a dropped connection can heal, as one does after a Mac wakes, without a false alarm.
const LOST_AFTER_MS = 2500;
const RETRY_MIN_MS = 1000, RETRY_MAX_MS = 30000;
const REFUSED_NAME = 422;

sendAnswer = async body => {
  const response = await fetch(withToken("answer"), { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  const reply = await response.json();
  return { ok: response.ok, refusal: response.status === REFUSED_NAME ? reply.error : undefined };
};

let gone = null, ended = false, lostTimer = null, retry = RETRY_MIN_MS;

function connect() {
  const stream = new EventSource(withToken("events"));
  stream.onmessage = message => {
    const frame = JSON.parse(message.data);
    retry = RETRY_MIN_MS;
    clearTimeout(lostTimer);
    lostTimer = null;
    gone = frame.gone || gone;
    ended = frame.ended;
    draw(frame);
    if (ended) stream.close();
  };
  stream.onerror = () => {
    stream.close();
    if (ended) return;
    // Closing without `ended` means the server went away. The words came with the last frame, so this needs no request.
    if (gone && lostTimer === null) lostTimer = setTimeout(() => draw({ seq, lost: true, screen: gone }), LOST_AFTER_MS);
    setTimeout(connect, retry);
    retry = Math.min(retry * 2, RETRY_MAX_MS);
  };
}

startThreads();
ready.then(connect);
