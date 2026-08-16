const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const { createWorkspaceWatcher } = require("../Electron/workspace-watcher.cjs");

function nextChange(label, register, timeout = 4000) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`${label}文件系统事件超时`)), timeout);
    register(() => {
      clearTimeout(timer);
      resolve();
    });
  });
}

test("桌面工作区监听新增、重命名和删除事件", async t => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "mory-watcher-"));
  const callbacks = [];
  const watcher = createWorkspaceWatcher({
    debounceMs: 40,
    onChange: () => callbacks.shift()?.()
  });
  t.after(async () => {
    watcher.stop();
    await fs.rm(root, { recursive: true, force: true });
  });
  watcher.start(root);

  const first = path.join(root, "第一篇.md");
  const created = nextChange("新增", resolve => callbacks.push(resolve));
  await fs.writeFile(first, "# 第一篇", "utf8");
  await created;

  const renamed = path.join(root, "已重命名.md");
  const moved = nextChange("重命名", resolve => callbacks.push(resolve));
  await fs.rename(first, renamed);
  await moved;

  const removed = nextChange("删除", resolve => callbacks.push(resolve));
  await fs.unlink(renamed);
  await removed;

  assert.equal(watcher.root, path.resolve(root));
});
