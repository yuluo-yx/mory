// Materialize lazy clipboard data before native paste replaces its source.
async function snapshotClipboard(clipboard, ClipboardItem) {
  const items = await clipboard.read();
  // Empty system clipboards can expose an item with no MIME representations.
  return Promise.all(items.filter(item => item.types.length > 0).map(async item =>
    new ClipboardItem(Object.fromEntries(await Promise.all(item.types.map(async type =>
      [type, await item.getType(type)]))))));
}

module.exports = { snapshotClipboard };
