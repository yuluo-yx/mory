const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const { createThemeManager, themeID } = require("../Electron/themes.cjs");

const {
  createWorkspaceManager,
  importImage,
  listDocuments,
  loadDocumentAssets,
  markdownImagePaths,
  readWorkspaceDocuments,
  relocateDocumentAssets,
  sanitizeSegment,
  validateWorkspace
} = require("../Electron/workspaces.cjs");

async function fixture(t) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "mory-workspace-"));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  return root;
}

test("首次启动创建默认本地工作区", async t => {
  const root = await fixture(t);
  const local = path.join(root, "documents");
  const manager = createWorkspaceManager({ userDataPath: path.join(root, "config"), defaultRoot: local, sidecarPath: () => "missing" });
  const state = await manager.initialize();
  assert.equal(state.workspaces.length, 1);
  assert.equal(state.workspaces[0].provider, "local");
  assert.equal(manager.activeRoot(), local);
});

test("不同工作区保留独立凭证但不向渲染层回传秘密", async t => {
  const root = await fixture(t);
  const manager = createWorkspaceManager({ userDataPath: path.join(root, "config"), defaultRoot: path.join(root, "local"), sidecarPath: () => "missing" });
  await manager.initialize();
  let state = await manager.save({
    name: "站点仓库", provider: "github", repository: "owner/site", branch: "main", token: "github-secret"
  });
  const github = state.workspaces.find(item => item.provider === "github");
  assert.equal(github.token, undefined);
  assert.equal(github.tokenConfigured, true);
  state = await manager.save({ id: github.id, name: "站点仓库 2", provider: "github", repository: "owner/site", branch: "main" });
  assert.equal(state.workspaces.find(item => item.id === github.id).tokenConfigured, true);
});

test("工作区可切换、删除，本地同步不调用远端侧车", async t => {
  const root = await fixture(t);
  const manager = createWorkspaceManager({ userDataPath: path.join(root, "config"), defaultRoot: path.join(root, "local-a"), sidecarPath: () => "missing" });
  const initial = await manager.initialize();
  const second = await manager.save({ name: "本地 B", provider: "local", localPath: path.join(root, "local-b") });
  assert.equal(second.workspaces.length, 2);
  const activated = await manager.activate(initial.activeId);
  assert.equal(activated.activeId, initial.activeId);
  assert.deepEqual(await manager.sync("pull"), { files: 0, bytes: 0, local: true });
  const removed = await manager.remove(second.workspaces.find(item => item.name === "本地 B").id);
  assert.equal(removed.workspaces.length, 1);
  await assert.rejects(() => manager.remove(initial.activeId), /至少保留/);
  await assert.rejects(() => manager.activate("missing"), /不存在/);
});

test("S3、S4、OSS 和 SFTP 校验各自凭证", () => {
  for (const provider of ["s3", "s4", "oss"]) {
    assert.doesNotThrow(() => validateWorkspace({ provider, endpoint: provider === "s4" ? "https://s4.example" : "", region: "cn-test", bucket: "docs", accessKeyId: "id", accessKeySecret: "secret" }));
  }
  assert.doesNotThrow(() => validateWorkspace({ provider: "sftp", host: "server", username: "user", password: "pass", remotePath: "/docs" }));
  assert.throws(() => validateWorkspace({ provider: "s3" }), /对象存储/);
  assert.throws(() => validateWorkspace({ provider: "sftp", host: "server" }), /SFTP/);
});

test("图片按文稿名称归档并可内嵌加载", async t => {
  const root = await fixture(t);
  const documentPath = path.join(root, "文章.md");
  const result = await importImage({
    root,
    documentPath,
    documentName: "文章.md",
    name: "封面 图.png",
    mime: "image/png",
    data: Buffer.from("png-data").toString("base64")
  });
  assert.equal(result.relative, "文章/封面-图.png");
  assert.equal(await fs.readFile(path.join(root, result.relative), "utf8"), "png-data");
  const assets = await loadDocumentAssets(documentPath, `![封面](${result.relative})`);
  assert.match(assets[result.relative], /^data:image\/png;base64,/);
});

test("草稿保存后图片目录跟随文稿重命名", async t => {
  const root = await fixture(t);
  await fs.mkdir(path.join(root, "未命名"), { recursive: true });
  await fs.writeFile(path.join(root, "未命名", "图.png"), "image");
  const markdown = await relocateDocumentAssets({
    root, markdown: "![图](未命名/图.png)", oldPath: "", oldName: "未命名.md", newPath: path.join(root, "正式文章.md")
  });
  assert.equal(markdown, "![图](正式文章/图.png)");
  assert.equal(await fs.readFile(path.join(root, "正式文章", "图.png"), "utf8"), "image");
});

test("递归列出文稿并忽略资源目录中的非文稿", async t => {
  const root = await fixture(t);
  await fs.mkdir(path.join(root, "专题", "文章"), { recursive: true });
  await fs.writeFile(path.join(root, "专题", "文章.md"), "# 文章");
  await fs.writeFile(path.join(root, "专题", "文章", "image.png"), "image");
  assert.deepEqual(await listDocuments(root), [{ name: path.join("专题", "文章.md"), path: path.join(root, "专题", "文章.md") }]);
});

test("知识图谱数据读取工作区文稿内容", async t => {
  const root = await fixture(t);
  await fs.writeFile(path.join(root, "入口.md"), "# 入口\n[[目标]]");
  assert.deepEqual(await readWorkspaceDocuments(root), [{ name: "入口.md", path: path.join(root, "入口.md"), markdown: "# 入口\n[[目标]]" }]);
});

test("Markdown 图片路径与文件名清理保持稳定", () => {
  assert.deepEqual(markdownImagePaths("![a](文稿/a.png)\n![b](https://x/b.png)"), ["文稿/a.png"]);
  assert.equal(sanitizeSegment("  封面 图:*  "), "-封面-图-");
});

test("用户主题导入后保持稳定标识并内联相对资源", async t => {
  const root = await fixture(t);
  const source = path.join(root, "纸张.css");
  await fs.writeFile(path.join(root, "纹理.svg"), "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>");
  await fs.writeFile(source, '.write{background-image:url("纹理.svg")}');
  const manager = createThemeManager({ userDataPath: path.join(root, "config") });
  const themes = await manager.importFile(source);
  await fs.copyFile(path.join(root, "纹理.svg"), path.join(manager.directory, "纹理.svg"));
  const refreshed = await manager.list();
  assert.equal(themes[0].id, themeID("纸张.css"));
  assert.equal(refreshed[0].name, "纸张");
  assert.match(refreshed[0].css, /data:image\/svg\+xml;base64,/);
});
