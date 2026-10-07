const assert = require("node:assert/strict");
const test = require("node:test");
const { setDefaultMarkdownApp } = require("../Electron/default-app.cjs");

test("Windows opens default apps without claiming the association changed", async () => {
  const opened = [];
  const result = await setDefaultMarkdownApp("win32", { openExternal: async url => opened.push(url) });
  assert.deepEqual(opened, ["ms-settings:defaultapps"]);
  assert.deepEqual(result, { status: "settings" });
});

test("Windows propagates settings launch failures", async () => {
  await assert.rejects(setDefaultMarkdownApp("win32", { openExternal: async () => { throw new Error("Launch failed"); } }), /Launch failed/);
});

test("macOS compatibility host requests Finder instructions", async () => {
  assert.deepEqual(await setDefaultMarkdownApp("darwin", {}), { status: "finder" });
});

test("unsupported hosts cannot claim success", async () => {
  await assert.rejects(setDefaultMarkdownApp("linux", {}), /not supported/);
});
