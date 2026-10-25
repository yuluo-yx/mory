const fs = require("node:fs/promises");

/**
 * Polls the document shown in the editor and reports its disk content after another program edits it.
 * Polling file metadata is used instead of filesystem events because many editors save by writing a
 * temporary file and renaming it, which detaches event watchers from the original file.
 * The renderer compares the reported content with its last known disk version, so a report for the
 * application's own save is harmless.
 */
function createDocumentDiskMonitor({ onChange, onError = () => {}, intervalMs = 1000 }) {
  let trackedPath = "";
  let lastStamp = "";
  let timer = null;
  let checking = false;
  let recheck = false;

  async function check({ force = false } = {}) {
    if (checking) {
      // A newer document selection must still be reported after the in-flight check finishes.
      recheck = recheck || force;
      return;
    }
    checking = true;
    const filePath = trackedPath;
    try {
      if (!filePath) return;
      let stat;
      try { stat = await fs.stat(filePath); }
      catch { return; } // Deleted or temporarily replaced files are reconciled by the workspace scan.
      if (!stat.isFile()) return;
      const stamp = `${stat.mtimeMs}:${stat.size}`;
      if (!force && stamp === lastStamp) return;
      const markdown = await fs.readFile(filePath, "utf8");
      if (filePath !== trackedPath) return;
      lastStamp = stamp;
      await onChange({ path: filePath, markdown });
    } catch (error) {
      onError(error);
    } finally {
      checking = false;
      if (recheck) {
        recheck = false;
        void check({ force: true });
      }
    }
  }

  return {
    /** Starts tracking a document path, or stops tracking for drafts without a path. */
    track(filePath) {
      const next = filePath ? String(filePath) : "";
      trackedPath = next;
      lastStamp = "";
      clearInterval(timer);
      timer = null;
      if (!next) return Promise.resolve();
      timer = setInterval(() => void check(), intervalMs);
      timer.unref?.();
      // Report immediately so changes made while the document was in the background are not missed.
      return check({ force: true });
    },
    check,
    stop() {
      trackedPath = "";
      lastStamp = "";
      clearInterval(timer);
      timer = null;
    },
    get path() { return trackedPath; }
  };
}

module.exports = { createDocumentDiskMonitor };
