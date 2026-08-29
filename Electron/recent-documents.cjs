function listRecentDocuments(application, platform, existsSync, limit = 10) {
  if (!["darwin", "win32"].includes(platform) || typeof application.getRecentDocuments !== "function") return [];
  return application.getRecentDocuments().filter(filePath => existsSync(filePath)).slice(0, limit);
}

function addRecentDocument(application, platform, filePath) {
  if (!["darwin", "win32"].includes(platform) || typeof application.addRecentDocument !== "function") return false;
  application.addRecentDocument(filePath);
  return true;
}

function clearRecentDocuments(application) {
  if (typeof application.clearRecentDocuments !== "function") return false;
  application.clearRecentDocuments();
  return true;
}

module.exports = { addRecentDocument, clearRecentDocuments, listRecentDocuments };
