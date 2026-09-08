const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

assert.equal(process.platform, "darwin", "This smoke test requires macOS");
const application = path.resolve(__dirname, "../dist/macos/Mory.app");
const client = path.join(application, "Contents/Resources/bin/mory");
assert.ok(fs.existsSync(client), "Build the native app with npm run build:mac first");
const root = fs.mkdtempSync(path.join(os.tmpdir(), "mory-cli-export-"));
const sourceName = "-\u6587\u6863 with spaces.md";
const source = path.join(root, sourceName);
const markdown = "# CLI export\n\nA real **Markdown** document.\n\n```js\nconst value = 42;\n```\n";
let scenarios = 0;

function run(args, expectedError) {
  const result = spawnSync(client, ["--app", application, ...args], {
    cwd: root, encoding: "utf8", timeout: 60000
  });
  assert.ifError(result.error);
  if (expectedError) {
    assert.notEqual(result.status, 0, "An invalid export must fail");
    assert.match(result.stderr, expectedError);
    assert.equal(result.stdout, "", "A failed export must not report a destination");
  } else {
    assert.equal(result.status, 0, result.stderr);
  }
  scenarios += 1;
  return result;
}

try {
  fs.writeFileSync(source, markdown);
  const output = path.join(root, "Export folder");
  fs.mkdirSync(output);
  for (const [format, extension, signature] of [
    ["html", ".html", Buffer.from("<!doctype html>")],
    ["pdf", ".pdf", Buffer.from("%PDF-")],
    ["png", ".png", Buffer.from([0x89, 0x50, 0x4e, 0x47])],
    ["JPG", ".jpg", Buffer.from([0xff, 0xd8, 0xff])]
  ]) {
    const args = ["export", "--path", output];
    if (format !== "pdf") args.push("--format", format);
    const result = run([...args, "--", sourceName]);
    const destination = result.stdout.trim();
    assert.equal(path.dirname(destination), output);
    assert.equal(path.extname(destination), extension);
    const exported = fs.readFileSync(destination);
    assert.ok(exported.length > signature.length);
    assert.deepEqual(exported.subarray(0, signature.length), signature);
    run([...args, "--", sourceName], /output already exists/);
    assert.deepEqual(fs.readFileSync(destination), exported);
    if (format === "html") {
      fs.writeFileSync(destination, "previous export");
      run([...args, "--force", "--", sourceName]);
      assert.match(fs.readFileSync(destination, "utf8"), /CLI export/);
    }
    fs.unlinkSync(destination);
    fs.mkdirSync(destination);
    run([...args, "--force", "--", sourceName], /not a regular file/);
    fs.rmdirSync(destination);
    for (const target of [source, path.join(root, "missing")]) {
      fs.symlinkSync(target, destination);
      run([...args, "--force", "--", sourceName], /not a regular file/);
      fs.unlinkSync(destination);
    }
    fs.linkSync(source, destination);
    run([...args, "--force", "--", sourceName], /refers to the source document/);
    fs.unlinkSync(destination);
    assert.equal(fs.readFileSync(source, "utf8"), markdown);
  }
  run(["export", "--format", "docx", "--", sourceName], /unsupported export format/);
  run(["export", "--path", "missing folder", "--", sourceName], /open output directory/);
  process.stdout.write(`Native CLI export smoke passed: ${scenarios} scenarios (HTML, PDF, PNG, JPG, collisions, and source preservation).\n`);
} finally {
  fs.rmSync(root, { recursive: true, force: true });
}
