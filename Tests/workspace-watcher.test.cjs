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
      reject(new Error(`${label}Timed out waiting for a file-system event; observed snapshots=${JSON.stringify(snapshots)}`));
    }, timeout);
    waiters.add(waiter);
  });
}

test("watches desktop workspaces for create, rename, and delete events", async t => {
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

  const first = path.join(root, "\u7B2C\u4E00\u7BC7.md");
  const created = waitForSnapshot("\u65B0\u589E", snapshots, waiters, files => files.includes("\u7B2C\u4E00\u7BC7.md"));
  await fs.writeFile(first, "# \u7B2C\u4E00\u7BC7", "utf8");
  await created;

  const renamed = path.join(root, "\u5DF2\u91CD\u547D\u540D.md");
  const moved = waitForSnapshot(
    "\u91CD\u547D\u540D",
    snapshots,
    waiters,
    files => files.includes("\u5DF2\u91CD\u547D\u540D.md") && !files.includes("\u7B2C\u4E00\u7BC7.md"),
  );
  await fs.rename(first, renamed);
  await moved;

  const removed = waitForSnapshot("\u5220\u9664", snapshots, waiters, files => files.length === 0);
  await fs.unlink(renamed);
  await removed;

  assert.equal(watcher.root, path.resolve(root));
  assert.deepEqual(snapshots.at(-1), []);
});

test("ignores hidden workspace paths such as Git metadata", () => {
  const { isRelevantWorkspaceEvent } = require("../Electron/workspace-watcher.cjs");
  assert.equal(isRelevantWorkspaceEvent(".git/index"), false);
  assert.equal(isRelevantWorkspaceEvent("notes\\.DS_Store"), false);
  assert.equal(isRelevantWorkspaceEvent("notes/today.md"), true);
  assert.equal(isRelevantWorkspaceEvent(null), true);
});

test("signature polling refreshes only when the workspace signature changes", async t => {
  let current = "a";
  let refreshes = 0;
  const watcher = createWorkspaceWatcher({
    debounceMs: 5,
    pollIntervalMs: 20,
    signature: async () => current,
    onChange: () => { refreshes += 1; }
  });
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "mory-signature-"));
  t.after(async () => {
    watcher.stop();
    await fs.rm(root, { recursive: true, force: true });
  });
  watcher.start(root);
  await new Promise(resolve => setTimeout(resolve, 200));
  // The first poll publishes the initial state; unchanged signatures must not repeat it.
  assert.equal(refreshes, 1);
  current = "b";
  await new Promise(resolve => setTimeout(resolve, 120));
  assert.equal(refreshes, 2);
});

test("workspace signatures track visible entries and skip hidden metadata", async t => {
  const { workspaceSignature } = require("../Electron/workspaces.cjs");
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "mory-signature-tree-"));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  await fs.mkdir(path.join(root, "notes"));
  await fs.writeFile(path.join(root, "notes", "a.md"), "# A");
  const initial = await workspaceSignature(root);
  await fs.mkdir(path.join(root, ".git"));
  await fs.writeFile(path.join(root, ".git", "HEAD"), "ref");
  assert.equal(await workspaceSignature(root), initial);
  await fs.writeFile(path.join(root, "notes", "a.md"), "# A changed");
  assert.notEqual(await workspaceSignature(root), initial);
});
