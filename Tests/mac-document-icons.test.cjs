const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const { spawnSync } = require("node:child_process");
const test = require("node:test");

const root = path.join(__dirname, "..");
const markdownExtensions = ["md", "markdown", "mmd", "mdown", "mkd"];
const iconRepresentations = [
  ["icon_16x16.png", 16], ["icon_16x16@2x.png", 32],
  ["icon_32x32.png", 32], ["icon_32x32@2x.png", 64],
  ["icon_128x128.png", 128], ["icon_128x128@2x.png", 256],
  ["icon_256x256.png", 256], ["icon_256x256@2x.png", 512],
  ["icon_512x512.png", 512], ["icon_512x512@2x.png", 1024]
];

function assertIconRepresentations(iconPath, iconsetPath) {
  const result = spawnSync("/usr/bin/iconutil", ["--convert", "iconset", "--output", iconsetPath, iconPath], { encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr || "The system must decode the ICNS resource");
  for (const [filename, size] of iconRepresentations) {
    const png = fs.readFileSync(path.join(iconsetPath, filename));
    assert.equal(png.subarray(0, 8).toString("hex"), "89504e470d0a1a0a");
    assert.equal(png.readUInt32BE(16), size, `${filename} must preserve its pixel width`);
    assert.equal(png.readUInt32BE(20), size, `${filename} must preserve its pixel height`);
  }
}

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
    const temporary = fs.mkdtempSync(path.join(os.tmpdir(), "mory-document-icon-"));
    try {
      assertIconRepresentations(path.join(bundle, "Contents", "Resources", document.CFBundleTypeIconFile), path.join(temporary, "Document.iconset"));
    } finally {
      fs.rmSync(temporary, { recursive: true, force: true });
    }
  }
});

test("the icon regression check rejects the former hand-packed ICNS without Retina representations", {
  skip: process.platform !== "darwin" || !process.env.MORY_TEST_APP_BUNDLE
}, () => {
  const bundle = path.resolve(root, process.env.MORY_TEST_APP_BUNDLE);
  const temporary = fs.mkdtempSync(path.join(os.tmpdir(), "mory-legacy-icon-"));
  try {
    const iconset = path.join(temporary, "Source.iconset");
    assertIconRepresentations(path.join(bundle, "Contents", "Resources", "icon.icns"), iconset);
    const legacy = path.join(temporary, "legacy.icns");
    const result = spawnSync(process.execPath, [path.join(__dirname, "fixtures", "legacy-png-icns.mjs"), iconset, legacy], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    assert.throws(() => assertIconRepresentations(legacy, path.join(temporary, "Legacy.iconset")));
  } finally {
    fs.rmSync(temporary, { recursive: true, force: true });
  }
});
