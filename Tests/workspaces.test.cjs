const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const { createThemeManager, themeID } = require("../Electron/themes.cjs");

const {
  compareDocumentsByCreation,
  copyWorkspaceEntry,
  createWorkspaceDirectory,
  createWorkspaceDocument,
  createWorkspaceManager,
  importImage,
  listDirectories,
  listDocuments,
  loadDocumentAssets,
  markdownImagePaths,
  moveWorkspaceEntry,
  renameWorkspaceEntry,
  readWorkspaceDocuments,
  readDocumentImage,
  relocateDocumentAssets,
  resolveWorkspaceDirectory,
  sanitizeSegment,
  storageWorkspace,
  validateWorkspace
} = require("../Electron/workspaces.cjs");

async function fixture(t) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "mory-workspace-"));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  return root;
}

test("creates a default local workspace on first launch", async t => {
  const root = await fixture(t);
  const local = path.join(root, "documents");
  const manager = createWorkspaceManager({ userDataPath: path.join(root, "config"), defaultRoot: local, sidecarPath: () => "missing" });
  const state = await manager.initialize();
  assert.equal(state.workspaces.length, 1);
  assert.equal(state.workspaces[0].provider, "local");
  assert.equal(state.workspaces[0].isImplicit, true);
  assert.equal(manager.activeRoot(), local);
  const configured = await manager.save({ ...state.workspaces[0], name: "\u660E\u786E\u9009\u62E9", localPath: local });
  assert.equal(configured.workspaces[0].isImplicit, false);
});

test("keeps credentials isolated per workspace without exposing secrets to the renderer", async t => {
  const root = await fixture(t);
  const manager = createWorkspaceManager({ userDataPath: path.join(root, "config"), defaultRoot: path.join(root, "local"), sidecarPath: () => "missing" });
  await manager.initialize();
  let state = await manager.save({
    name: "\u7AD9\u70B9\u4ED3\u5E93", provider: "github", repository: "owner/site", branch: "main", token: "github-secret"
  });
  const github = state.workspaces.find(item => item.provider === "github");
  assert.equal(github.token, undefined);
  assert.equal(github.tokenConfigured, true);
  state = await manager.save({ id: github.id, name: "\u7AD9\u70B9\u4ED3\u5E93 2", provider: "github", repository: "owner/site", branch: "main" });
  assert.equal(state.workspaces.find(item => item.id === github.id).tokenConfigured, true);
});

test("sends only the stable storage contract to the remote sidecar", () => {
  const workspace = storageWorkspace({
    id: "remote", name: "Repository", provider: "github", repository: "owner/site",
    branch: "main", token: "secret", localPath: "/cache/remote", isImplicit: false,
    tokenConfigured: true
  });
  assert.deepEqual(workspace, {
    id: "remote", name: "Repository", provider: "github", repository: "owner/site",
    branch: "main", token: "secret"
  });
  assert.equal("isImplicit" in workspace, false);
  assert.equal("localPath" in workspace, false);
  assert.equal("tokenConfigured" in workspace, false);
});

test("switches and removes workspaces without invoking a remote sidecar for local sync", async t => {
  const root = await fixture(t);
  const manager = createWorkspaceManager({ userDataPath: path.join(root, "config"), defaultRoot: path.join(root, "local-a"), sidecarPath: () => "missing" });
  const initial = await manager.initialize();
  const second = await manager.save({ name: "\u672C\u5730 B", provider: "local", localPath: path.join(root, "local-b") });
  assert.equal(second.workspaces.length, 2);
  const activated = await manager.activate(initial.activeId);
  assert.equal(activated.activeId, initial.activeId);
  assert.deepEqual(await manager.sync("pull"), { files: 0, bytes: 0, local: true });
  const removed = await manager.remove(second.workspaces.find(item => item.name === "\u672C\u5730 B").id);
  assert.equal(removed.workspaces.length, 1);
  await assert.rejects(() => manager.remove(initial.activeId), /\u81F3\u5C11\u4FDD\u7559/);
  await assert.rejects(() => manager.activate("missing"), /\u4E0D\u5B58\u5728/);
});

test("validates provider-specific credentials for S3, S4, OSS, and SFTP", () => {
  for (const provider of ["s3", "s4", "oss"]) {
    assert.doesNotThrow(() => validateWorkspace({ provider, endpoint: provider === "s4" ? "https://s4.example" : "", region: "cn-test", bucket: "docs", accessKeyId: "id", accessKeySecret: "secret" }));
  }
  assert.doesNotThrow(() => validateWorkspace({ provider: "sftp", host: "server", username: "user", password: "pass", remotePath: "/docs" }));
  assert.throws(() => validateWorkspace({ provider: "s3" }), /\u5BF9\u8C61\u5B58\u50A8/);
  assert.throws(() => validateWorkspace({ provider: "sftp", host: "server" }), /SFTP/);
});

test("stores images by document name and loads them as embedded assets", async t => {
  const root = await fixture(t);
  const documentPath = path.join(root, "\u6587\u7AE0.md");
  const result = await importImage({
    root,
    documentPath,
    documentName: "\u6587\u7AE0.md",
    name: "\u5C01\u9762 \u56FE.png",
    mime: "image/png",
    data: Buffer.from("png-data").toString("base64")
  });
  assert.equal(result.relative, "\u6587\u7AE0/\u5C01\u9762-\u56FE.png");
  assert.equal(await fs.readFile(path.join(root, result.relative), "utf8"), "png-data");
  await fs.writeFile(path.join(root, "photo_1.jpg"), "avatar-data");
  await fs.mkdir(path.join(root, "img", "summary"), { recursive: true });
  await fs.writeFile(path.join(root, "img", "summary", "cover.jpg"), "root-asset");
  const assets = await loadDocumentAssets(documentPath, `![\u5C01\u9762](${result.relative})\n<img alt="avatar" src="./photo_1.jpg">\n![root](/img/summary/cover.jpg)`, root);
  assert.match(assets[result.relative], /^data:image\/png;base64,/);
  assert.match(assets["./photo_1.jpg"], /^data:image\/jpeg;base64,/);
  assert.match(assets["/img/summary/cover.jpg"], /^data:image\/jpeg;base64,/);
});

test("renames the image directory when a draft is saved under a document name", async t => {
  const root = await fixture(t);
  await fs.mkdir(path.join(root, "\u672A\u547D\u540D"), { recursive: true });
  await fs.writeFile(path.join(root, "\u672A\u547D\u540D", "\u56FE.png"), "image");
  const markdown = await relocateDocumentAssets({
    root, markdown: "![\u56FE](\u672A\u547D\u540D/\u56FE.png)", oldPath: "", oldName: "\u672A\u547D\u540D.md", newPath: path.join(root, "\u6B63\u5F0F\u6587\u7AE0.md")
  });
  assert.equal(markdown, "![\u56FE](\u6B63\u5F0F\u6587\u7AE0/\u56FE.png)");
  assert.equal(await fs.readFile(path.join(root, "\u6B63\u5F0F\u6587\u7AE0", "\u56FE.png"), "utf8"), "image");
});

test("lists documents recursively while ignoring non-documents in asset folders", async t => {
  const root = await fixture(t);
  await fs.mkdir(path.join(root, "\u4E13\u9898", "\u6587\u7AE0"), { recursive: true });
  await fs.writeFile(path.join(root, "\u4E13\u9898", "\u6587\u7AE0.md"), "# \u6587\u7AE0");
  await fs.writeFile(path.join(root, "\u4E13\u9898", "\u6587\u7AE0", "image.png"), "image");
  const documents = await listDocuments(root);
  assert.equal(documents.length, 1);
  assert.deepEqual({ name: documents[0].name, path: documents[0].path }, { name: path.join("\u4E13\u9898", "\u6587\u7AE0.md"), path: path.join(root, "\u4E13\u9898", "\u6587\u7AE0.md") });
  assert.ok(Number.isFinite(documents[0].createdAt));
  assert.equal(documents[0].size, Buffer.byteLength("# \u6587\u7AE0"));
  assert.ok(Number.isFinite(documents[0].updatedAt));
  assert.deepEqual(documents[0].images.map(image => image.relative), ["\u6587\u7AE0/image.png"]);
  assert.equal(documents[0].images[0].size, Buffer.byteLength("image"));
  assert.ok(Number.isFinite(documents[0].images[0].updatedAt));
  assert.deepEqual((await listDirectories(root)).map(directory => directory.name), ["\u4E13\u9898"]);
  const preview = await readDocumentImage(root, documents[0].images[0].path);
  assert.match(preview.dataURL, /^data:image\/png;base64,/);
  await assert.rejects(() => readDocumentImage(root, path.join(root, "\u4E13\u9898", "\u6587\u7AE0.md")), /\u56FE\u7247\u683C\u5F0F/);
});

test("creates nested workspace directories and rejects path traversal", async t => {
  const root = await fixture(t);
  const created = await createWorkspaceDirectory(root, "\u8D44\u6599/\u9879\u76EE A");
  assert.equal(created.name, path.join("\u8D44\u6599", "\u9879\u76EE A"));
  assert.equal((await fs.stat(created.path)).isDirectory(), true);
  assert.deepEqual((await listDirectories(root)).map(item => item.name), ["\u8D44\u6599", path.join("\u8D44\u6599", "\u9879\u76EE A")]);
  assert.throws(() => resolveWorkspaceDirectory(root, "../\u5916\u90E8"), /\u4E0D\u80FD\u5305\u542B|\u5FC5\u987B\u4F4D\u4E8E/);
  assert.throws(() => resolveWorkspaceDirectory(root, path.resolve(root, "\u7EDD\u5BF9\u8DEF\u5F84")), /\u76F8\u5BF9\u76EE\u5F55/);
});

test("creates, copies, and moves workspace entries with their document images", async t => {
  const root = await fixture(t);
  const sourceDirectory = path.join(root, "\u8D44\u6599");
  const targetDirectory = path.join(root, "\u5F52\u6863");
  await fs.mkdir(sourceDirectory, { recursive: true });
  await fs.mkdir(targetDirectory, { recursive: true });
  const document = await createWorkspaceDocument(root, sourceDirectory, "\u6587\u7AE0.md");
  await fs.mkdir(path.join(sourceDirectory, "\u6587\u7AE0"));
  await fs.writeFile(path.join(sourceDirectory, "\u6587\u7AE0", "\u5C01\u9762.png"), "image");

  const copied = await copyWorkspaceEntry(root, document.path, sourceDirectory);
  assert.equal(path.basename(copied.path), "\u6587\u7AE0 \u526F\u672C.md");
  assert.equal(await fs.readFile(path.join(sourceDirectory, "\u6587\u7AE0-\u526F\u672C", "\u5C01\u9762.png"), "utf8"), "image");

  const moved = await moveWorkspaceEntry(root, copied.path, targetDirectory);
  assert.equal(path.dirname(moved.path), targetDirectory);
  assert.equal(await fs.readFile(path.join(targetDirectory, "\u6587\u7AE0-\u526F\u672C", "\u5C01\u9762.png"), "utf8"), "image");
  await assert.rejects(() => moveWorkspaceEntry(root, moved.path, targetDirectory), /\u5DF2\u7ECF\u4F4D\u4E8E/);
});

test("renames workspace entries and keeps document image directories synchronized", async t => {
  const root = await fixture(t);
  const source = path.join(root, "\u539F\u6587.md");
  await fs.writeFile(source, "# \u539F\u6587\n\n![\u56FE](\u539F\u6587/\u5C01\u9762.png)");
  await fs.mkdir(path.join(root, "\u539F\u6587"));
  await fs.writeFile(path.join(root, "\u539F\u6587", "\u5C01\u9762.png"), "image");

  const renamed = await renameWorkspaceEntry(root, source, "\u65B0\u6587\u7A3F");
  assert.equal(renamed.name, "\u65B0\u6587\u7A3F.md");
  assert.equal(await fs.readFile(path.join(root, "\u65B0\u6587\u7A3F.md"), "utf8"), "# \u539F\u6587\n\n![\u56FE](\u65B0\u6587\u7A3F/\u5C01\u9762.png)");
  assert.equal(await fs.readFile(path.join(root, "\u65B0\u6587\u7A3F", "\u5C01\u9762.png"), "utf8"), "image");
  await assert.rejects(() => fs.access(source));
  await assert.rejects(() => renameWorkspaceEntry(root, renamed.path, "\u65B0\u6587\u7A3F"), /\u6CA1\u6709\u53D8\u5316/);
  await assert.rejects(() => renameWorkspaceEntry(root, renamed.path, "../\u8D8A\u754C.md"), /\u8DEF\u5F84\u5206\u9694\u7B26/);
  await assert.rejects(() => renameWorkspaceEntry(root, renamed.path, "..\\\u8D8A\u754C.md"), /\u8DEF\u5F84\u5206\u9694\u7B26/);
  await fs.writeFile(path.join(root, "\u51B2\u7A81.md"), "existing");
  await assert.rejects(() => renameWorkspaceEntry(root, renamed.path, "\u51B2\u7A81.md"), /\u540C\u540D\u6761\u76EE/);

  const directory = path.join(root, "\u65E7\u76EE\u5F55");
  await fs.mkdir(directory);
  await fs.writeFile(path.join(directory, "\u6587\u7AE0.md"), "nested");
  const renamedDirectory = await renameWorkspaceEntry(root, directory, "\u65B0\u76EE\u5F55");
  assert.equal(renamedDirectory.name, "\u65B0\u76EE\u5F55");
  assert.equal(await fs.readFile(path.join(root, "\u65B0\u76EE\u5F55", "\u6587\u7AE0.md"), "utf8"), "nested");
});

test("copies directory subtrees and rejects copy or move operations into descendants", async t => {
  const root = await fixture(t);
  const source = path.join(root, "\u8D44\u6599");
  const child = path.join(source, "\u9879\u76EE");
  await fs.mkdir(child, { recursive: true });
  await fs.writeFile(path.join(child, "\u8BF4\u660E.md"), "# \u8BF4\u660E");
  await assert.rejects(() => copyWorkspaceEntry(root, source, child), /\u81EA\u8EAB\u6216\u5B50\u76EE\u5F55/);
  await assert.rejects(() => moveWorkspaceEntry(root, source, child), /\u81EA\u8EAB\u6216\u5B50\u76EE\u5F55/);
  const copied = await copyWorkspaceEntry(root, source, root);
  assert.equal(copied.isDirectory, true);
  assert.equal(await fs.readFile(path.join(root, "\u8D44\u6599 \u526F\u672C", "\u9879\u76EE", "\u8BF4\u660E.md"), "utf8"), "# \u8BF4\u660E");
});

test("sorts documents by creation time with names as a stable tie-breaker", () => {
  const documents = [
    { name: "12.md", path: "/12.md", createdAt: 30 },
    { name: "02.md", path: "/02.md", createdAt: 10 },
    { name: "01.md", path: "/01.md", createdAt: 10 }
  ];
  assert.deepEqual(documents.sort(compareDocumentsByCreation).map(item => item.name), ["01.md", "02.md", "12.md"]);
});

test("loads workspace document content for the knowledge graph", async t => {
  const root = await fixture(t);
  await fs.writeFile(path.join(root, "\u5165\u53E3.md"), "# \u5165\u53E3\n[[\u76EE\u6807]]");
  const documents = await readWorkspaceDocuments(root);
  assert.equal(documents.length, 1);
  assert.deepEqual(
    { name: documents[0].name, path: documents[0].path, markdown: documents[0].markdown },
    { name: "\u5165\u53E3.md", path: path.join(root, "\u5165\u53E3.md"), markdown: "# \u5165\u53E3\n[[\u76EE\u6807]]" }
  );
  assert.ok(Number.isFinite(documents[0].createdAt));
});

test("keeps Markdown image paths and filename sanitization stable", () => {
  assert.deepEqual(
    markdownImagePaths("![a](\u6587\u7A3F/a.png)\n<img alt='avatar' src='./photo_1.jpg'>\n![b](https://x/b.png)"),
    ["\u6587\u7A3F/a.png", "./photo_1.jpg"]
  );
  assert.equal(sanitizeSegment("  \u5C01\u9762 \u56FE:*  "), "-\u5C01\u9762-\u56FE-");
});

test("keeps imported theme identifiers stable and inlines relative assets", async t => {
  const root = await fixture(t);
  const source = path.join(root, "\u7EB8\u5F20.css");
  await fs.writeFile(path.join(root, "\u7EB9\u7406.svg"), "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>");
  await fs.writeFile(source, '.write{background-image:url("\u7EB9\u7406.svg")}');
  const manager = createThemeManager({ userDataPath: path.join(root, "config") });
  const themes = await manager.importFile(source);
  await fs.copyFile(path.join(root, "\u7EB9\u7406.svg"), path.join(manager.directory, "\u7EB9\u7406.svg"));
  const refreshed = await manager.list();
  assert.equal(themes[0].id, themeID("\u7EB8\u5F20.css"));
  assert.equal(refreshed[0].name, "\u7EB8\u5F20");
  assert.match(refreshed[0].css, /data:image\/svg\+xml;base64,/);
});

test("persists a custom theme directory across restarts", async t => {
  const root = await fixture(t);
  const userDataPath = path.join(root, "config");
  const selectedDirectory = path.join(root, "\u81EA\u5B9A\u4E49\u4E3B\u9898");
  let manager = createThemeManager({ userDataPath });
  const changed = await manager.setDirectory(selectedDirectory);
  assert.equal(changed.directory, selectedDirectory);
  await fs.writeFile(path.join(selectedDirectory, "custom.css"), "#write{color:teal}");
  manager = createThemeManager({ userDataPath });
  await manager.initialize();
  assert.equal(manager.directory, selectedDirectory);
  assert.equal((await manager.list())[0].name, "custom");
});
