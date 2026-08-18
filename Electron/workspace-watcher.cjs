const fs = require("node:fs");
const path = require("node:path");

/**
 * Tracks the active workspace with Node's filesystem watcher.
 * Recursive events on Windows and macOS are debounced into one atomic snapshot refresh.
 */
function createWorkspaceWatcher({ onChange, onError = () => {}, debounceMs = 180, pollIntervalMs = 1500 }) {
  let watcher = null;
  let watchedRoot = "";
  let refreshTimer = null;
  let pollTimer = null;

  function scheduleRefresh() {
    clearTimeout(refreshTimer);
    refreshTimer = setTimeout(() => {
      refreshTimer = null;
      Promise.resolve(onChange()).catch(onError);
    }, debounceMs);
  }

  function stop() {
    clearTimeout(refreshTimer);
    clearInterval(pollTimer);
    refreshTimer = null;
    pollTimer = null;
    watcher?.close();
    watcher = null;
    watchedRoot = "";
  }

  function start(root) {
    const nextRoot = path.resolve(root);
    if (watcher && watchedRoot === nextRoot) return;
    stop();
    watchedRoot = nextRoot;
    // Operating systems may coalesce or omit fs.watch events; periodic polling guarantees convergence.
    pollTimer = setInterval(scheduleRefresh, pollIntervalMs);
    pollTimer.unref?.();
    try {
      watcher = fs.watch(nextRoot, { recursive: true }, scheduleRefresh);
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

module.exports = { createWorkspaceWatcher };
