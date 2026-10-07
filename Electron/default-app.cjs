async function setDefaultMarkdownApp(platform, shell) {
  if (platform === "win32") {
    await shell.openExternal("ms-settings:defaultapps");
    return { status: "settings" };
  }
  if (platform === "darwin") return { status: "finder" };
  throw new Error("Default Markdown app settings are not supported on this platform");
}

module.exports = { setDefaultMarkdownApp };
