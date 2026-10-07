const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { spawnSync } = require("node:child_process");
const test = require("node:test");

const root = path.join(__dirname, "..");
const markdownExtensions = ["md", "markdown", "mmd", "mdown", "mkd"];

function readPlist(filename) {
  const result = spawnSync("/usr/bin/plutil", ["-convert", "json", "-o", "-", filename], { encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr || "The bundle property list must be valid");
  return JSON.parse(result.stdout);
}

function assertDocumentIcons(plist) {
  const documents = plist.CFBundleDocumentTypes;
  for (const extension of markdownExtensions) {
    const matches = documents.filter(document => document.CFBundleTypeExtensions?.includes(extension));
    assert.equal(matches.length, 1, `${extension} must have exactly one document declaration`);
    assert.equal(matches[0].CFBundleTypeIconFile, "icon.icns", `${extension} must use the Mory document icon`);
    assert.equal(matches[0].CFBundleTypeRole, "Editor");
    assert.equal(matches[0].LSHandlerRank, "Alternate", "Declaring an icon must not claim ownership of Markdown");
    assert.ok(!matches[0].CFBundleTypeExtensions.includes("txt"), "Plain text must remain separate from Markdown");
  }
  const plainText = documents.find(document => document.CFBundleTypeExtensions?.includes("txt"));
  assert.ok(plainText, "Plain text must remain supported");
  assert.equal(plainText.CFBundleTypeRole, "Editor");
  assert.equal(plainText.CFBundleTypeIconFile, undefined, "Do not replace plain text icons with Markdown artwork");
}

test("native Markdown documents declare their icon without changing plain text handling", { skip: process.platform !== "darwin" }, () => {
  assertDocumentIcons(readPlist(path.join(root, "macOS", "Info.plist")));
});

test("the packaged macOS app includes the declared full-resolution document icon", {
  skip: process.platform !== "darwin" || !process.env.MORY_TEST_APP_BUNDLE
}, () => {
  const bundle = path.resolve(root, process.env.MORY_TEST_APP_BUNDLE);
  const plist = readPlist(path.join(bundle, "Contents", "Info.plist"));
  assertDocumentIcons(plist);
  const source = readPlist(path.join(root, "macOS", "Info.plist"));
  assert.deepEqual(plist.CFBundleDocumentTypes, source.CFBundleDocumentTypes);
  assert.equal(plist.CFBundleShortVersionString, source.CFBundleShortVersionString);
  for (const document of plist.CFBundleDocumentTypes.filter(document => document.CFBundleTypeIconFile)) {
    const icon = fs.readFileSync(path.join(bundle, "Contents", "Resources", document.CFBundleTypeIconFile));
    assert.equal(icon.toString("ascii", 0, 4), "icns");
    assert.equal(icon.readUInt32BE(4), icon.length);
    assert.deepEqual(icon, fs.readFileSync(path.join(root, ".build", "icons", "icon.icns")));
    const representations = [];
    for (let offset = 8; offset < icon.length;) {
      assert.ok(offset + 8 <= icon.length, "The ICNS chunk header must be complete");
      const length = icon.readUInt32BE(offset + 4);
      assert.ok(length >= 8 && offset + length <= icon.length, "The ICNS chunk must be complete");
      representations.push(icon.toString("ascii", offset, offset + 4));
      offset += length;
    }
    assert.deepEqual(representations, ["icp4", "icp5", "icp6", "ic07", "ic08", "ic09", "ic10"]);
  }
});
