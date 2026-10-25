const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const { createDocumentDiskMonitor } = require("../Electron/document-disk-monitor.cjs");

async function fixture(t) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "mory-disk-monitor-"));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  return root;
}

function recorder() {
  const changes = [];
  const waiters = new Set();
  return {
    changes,
    onChange(change) {
      changes.push(change);
      for (const waiter of waiters) if (waiter()) waiters.delete(waiter);
    },
    waitFor(predicate, timeout = 4000) {
      if (changes.some(predicate)) return Promise.resolve();
      return new Promise((resolve, reject) => {
        const timer = setTimeout(() => reject(new Error(`Timed out; changes=${JSON.stringify(changes)}`)), timeout);
        waiters.add(() => {
          if (!changes.some(predicate)) return false;
          clearTimeout(timer);
          resolve();
          return true;
        });
      });
    }
  };
}

test("reports the tracked document immediately and after an external in-place edit", async t => {
  const root = await fixture(t);
  const note = path.join(root, "note.md");
  await fs.writeFile(note, "first");
  const events = recorder();
  const monitor = createDocumentDiskMonitor({ onChange: events.onChange, intervalMs: 20 });
  t.after(() => monitor.stop());
  await monitor.track(note);
  assert.deepEqual(events.changes, [{ path: note, markdown: "first" }]);
  await fs.writeFile(note, "second edit");
  await events.waitFor(change => change.markdown === "second edit");
  const count = events.changes.length;
  await new Promise(resolve => setTimeout(resolve, 120));
  assert.equal(events.changes.length, count, "Unchanged files must not be reported repeatedly");
});

test("detects atomic rename saves used by other editors", async t => {
  const root = await fixture(t);
  const note = path.join(root, "note.md");
  await fs.writeFile(note, "original");
  const events = recorder();
  const monitor = createDocumentDiskMonitor({ onChange: events.onChange, intervalMs: 20 });
  t.after(() => monitor.stop());
  await monitor.track(note);
  const temporary = path.join(root, ".note.md.tmp");
  await fs.writeFile(temporary, "replaced atomically");
  await fs.rename(temporary, note);
  await events.waitFor(change => change.markdown === "replaced atomically");
});

test("switches documents, ignores missing files, and stops for drafts", async t => {
  const root = await fixture(t);
  const first = path.join(root, "first.md");
  const second = path.join(root, "second.md");
  await fs.writeFile(first, "one");
  await fs.writeFile(second, "two");
  const events = recorder();
  const errors = [];
  const monitor = createDocumentDiskMonitor({ onChange: events.onChange, onError: error => errors.push(error), intervalMs: 20 });
  t.after(() => monitor.stop());
  await monitor.track(first);
  await monitor.track(second);
  assert.equal(monitor.path, second);
  assert.deepEqual(events.changes.map(change => change.markdown), ["one", "two"]);
  await fs.writeFile(first, "background edit");
  await new Promise(resolve => setTimeout(resolve, 100));
  assert.ok(!events.changes.some(change => change.markdown === "background edit"), "Inactive documents must not be reported");
  await fs.rm(second);
  await new Promise(resolve => setTimeout(resolve, 100));
  assert.deepEqual(errors, [], "A deleted document is not a monitor error");
  await monitor.track("");
  assert.equal(monitor.path, "");
  await fs.writeFile(second, "recreated");
  await new Promise(resolve => setTimeout(resolve, 100));
  assert.ok(!events.changes.some(change => change.markdown === "recreated"), "Stopped monitors must not report");
});

test("reports read failures through the error callback", async t => {
  const root = await fixture(t);
  const errors = [];
  const monitor = createDocumentDiskMonitor({
    onChange: () => { throw new Error("delivery failed"); },
    onError: error => errors.push(error.message),
    intervalMs: 20
  });
  t.after(() => monitor.stop());
  const note = path.join(root, "note.md");
  await fs.writeFile(note, "content");
  await monitor.track(note);
  assert.deepEqual(errors, ["delivery failed"]);
});
