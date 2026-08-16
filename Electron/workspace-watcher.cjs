const fs = require("node:fs");
const path = require("node:path");

/**
 * 使用 Node 官方文件系统监听器跟踪当前工作目录。
 * Windows 与 macOS 都支持 recursive；事件经过防抖后只触发一次原子快照刷新。
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
    // fs.watch 允许操作系统合并或遗漏事件；低频轮询保证 Finder 外部修改最终一定会刷新。
    pollTimer = setInterval(scheduleRefresh, pollIntervalMs);
    pollTimer.unref?.();
    try {
      watcher = fs.watch(nextRoot, { recursive: true }, scheduleRefresh);
      watcher.on("error", error => {
        onError(error);
        watcher?.close();
        watcher = null;
        // 原生监听失效后继续依靠轮询刷新；切换工作区时 start 会重建监听器。
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
