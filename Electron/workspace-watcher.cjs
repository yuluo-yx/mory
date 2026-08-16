const fs = require("node:fs");
const path = require("node:path");

/**
 * 使用 Node 官方文件系统监听器跟踪当前工作目录。
 * Windows 与 macOS 都支持 recursive；事件经过防抖后只触发一次原子快照刷新。
 */
function createWorkspaceWatcher({ onChange, onError = () => {}, debounceMs = 180 }) {
  let watcher = null;
  let watchedRoot = "";
  let refreshTimer = null;

  function scheduleRefresh() {
    clearTimeout(refreshTimer);
    refreshTimer = setTimeout(() => {
      refreshTimer = null;
      Promise.resolve(onChange()).catch(onError);
    }, debounceMs);
  }

  function stop() {
    clearTimeout(refreshTimer);
    refreshTimer = null;
    watcher?.close();
    watcher = null;
    watchedRoot = "";
  }

  function start(root) {
    const nextRoot = path.resolve(root);
    if (watcher && watchedRoot === nextRoot) return;
    stop();
    watchedRoot = nextRoot;
    try {
      watcher = fs.watch(nextRoot, { recursive: true }, scheduleRefresh);
      watcher.on("error", error => {
        onError(error);
        stop();
        // 根目录被移动或临时不可用时，刷新流程会重建目录并重新安装监听器。
        scheduleRefresh();
      });
    } catch (error) {
      stop();
      onError(error);
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
