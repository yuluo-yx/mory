const fs = require("node:fs");
const path = require("node:path");

/**
 * Reports whether a watcher event path can affect a workspace snapshot.
 * Hidden entries such as `.git` or `.DS_Store` never appear in snapshots, so their churn is ignored.
 */
function isRelevantWorkspaceEvent(filename) {
  if (!filename) return true;
  return !String(filename).split(/[\\/]+/).some(segment => segment.startsWith("."));
}

/**
 * Tracks the active workspace with Node's filesystem watcher.
 * Recursive events on Windows and macOS are debounced into one atomic snapshot refresh.
 * When `signature` is provided, periodic polling refreshes only after the cheap signature changes.
 */
function createWorkspaceWatcher({ onChange, onError = () => {}, debounceMs = 180, pollIntervalMs = 1500, signature = null }) {
  let watcher = null;
  let watchedRoot = "";
  let refreshTimer = null;
  let pollTimer = null;
  let lastSignature = null;
  let polling = false;

  async function captureSignature() {
    if (!signature) return;
    try { lastSignature = await signature(watchedRoot); }
    catch (error) { lastSignature = null; onError(error); }
  }

  function scheduleRefresh() {
    clearTimeout(refreshTimer);
    refreshTimer = setTimeout(() => {
      refreshTimer = null;
      // Record the state being published so the next poll does not repeat the same refresh.
      Promise.resolve(captureSignature()).then(() => onChange()).catch(onError);
    }, debounceMs);
  }

  async function poll() {
    if (!signature) { scheduleRefresh(); return; }
    if (polling) return;
    polling = true;
    const root = watchedRoot;
    try {
      const next = await signature(root);
      if (root === watchedRoot && next !== lastSignature) scheduleRefresh();
    } catch (error) {
      onError(error);
    } finally {
      polling = false;
    }
  }

  function stop() {
    clearTimeout(refreshTimer);
    clearInterval(pollTimer);
    refreshTimer = null;
    pollTimer = null;
    watcher?.close();
    watcher = null;
    watchedRoot = "";
    lastSignature = null;
  }

  function start(root) {
    const nextRoot = path.resolve(root);
    if (watcher && watchedRoot === nextRoot) return;
    stop();
    watchedRoot = nextRoot;
    // Operating systems may coalesce or omit fs.watch events; periodic polling guarantees convergence.
    pollTimer = setInterval(() => { void poll(); }, pollIntervalMs);
    pollTimer.unref?.();
    try {
      watcher = fs.watch(nextRoot, { recursive: true }, (_eventType, filename) => {
        if (isRelevantWorkspaceEvent(filename)) scheduleRefresh();
      });
      watcher.on("error", error => {
        onError(error);
        watcher?.close();
        watcher = null;
        // Continue polling after native watch failure; start recreates the watcher on workspace changes.
        scheduleRefresh();
      });
    } catch (error) {
      watcher = null;
      onError(error);
      scheduleRefresh();
    }
  }

  return {
    start,
    stop,
    scheduleRefresh,
    get root() { return watchedRoot; }
  };
}

module.exports = { createWorkspaceWatcher, isRelevantWorkspaceEvent };
