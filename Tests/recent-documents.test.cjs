const assert = require("node:assert/strict");
const test = require("node:test");

const { addRecentDocument, clearRecentDocuments, listRecentDocuments } = require("../Electron/recent-documents.cjs");

test("Electron recent items include existing documents and workspaces through operating-system APIs", () => {
  const calls = [];
  const application = {
    getRecentDocuments: () => ["/notes/first.md", "/notes/workspace", "/notes/missing.md", "/notes/second.md"],
    addRecentDocument: filePath => calls.push(["add", filePath]),
    clearRecentDocuments: () => calls.push(["clear"])
  };
  assert.deepEqual(
    listRecentDocuments(application, "darwin", filePath => !filePath.includes("missing")),
    ["/notes/first.md", "/notes/workspace", "/notes/second.md"]
  );
  assert.equal(addRecentDocument(application, "darwin", "/notes/first.md"), true);
  assert.equal(clearRecentDocuments(application), true);
  assert.deepEqual(calls, [["add", "/notes/first.md"], ["clear"]]);
});

test("Electron recent documents remain disabled on unsupported platforms", () => {
  const application = { getRecentDocuments: () => ["/notes/first.md"] };
  assert.deepEqual(listRecentDocuments(application, "linux", () => true), []);
  assert.equal(addRecentDocument(application, "linux", "/notes/first.md"), false);
});
