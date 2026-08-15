const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("moryNative", {
  platform: process.platform,
  send(payload) {
    ipcRenderer.send("mory:message", payload);
  }
});
