const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const { createWorkspaceWatcher } = require("../Electron/workspace-watcher.cjs");

function waitForSnapshot(label, snapshots, waiters, predicate, timeout = 4000) {
  if (snapshots.some(predicate)) return Promise.resolve();
  return new Promise((resolve, reject) => {
    const waiter = snapshot => {
      if (!predicate(snapshot)) return false;
      clearTimeout(timer);
      resolve();
      return true;
    };
    const timer = setTimeout(() => {
      waiters.delete(waiter);
      reject(new Error(`${label}文件系统事件超时；已观察快照=${JSON.stringify(snapshots)}`));
    }, timeout);
    waiters.add(waiter);
  });
}

test("桌面工作区监听新增、重命名和删除事件", async t => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "mory-watcher-"));
  const snapshots = [];
  const waiters = new Set();
  const watcher = createWorkspaceWatcher({
    debounceMs: 40,
    pollIntervalMs: 80,
    onChange: async () => {
      const snapshot = (await fs.readdir(root)).sort();
      snapshots.push(snapshot);
      for (const waiter of waiters) {
        if (waiter(snapshot)) waiters.delete(waiter);
      }
    }
  });
  t.after(async () => {
    watcher.stop();
    await fs.rm(root, { recursive: true, force: true });
  });
  watcher.start(root);

  const first = path.join(root, "第一篇.md");
  const created = waitForSnapshot("新增", snapshots, waiters, files => files.includes("第一篇.md"));
  await fs.writeFile(first, "# 第一篇", "utf8");
  await created;

  const renamed = path.join(root, "已重命名.md");
  const moved = waitForSnapshot(
    "重命名",
    snapshots,
    waiters,
    files => files.includes("已重命名.md") && !files.includes("第一篇.md"),
  );
  await fs.rename(first, renamed);
  await moved;

  const removed = waitForSnapshot("删除", snapshots, waiters, files => files.length === 0);
  await fs.unlink(renamed);
  await removed;

  assert.equal(watcher.root, path.resolve(root));
  assert.deepEqual(snapshots.at(-1), []);
});
