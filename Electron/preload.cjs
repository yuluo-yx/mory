const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("moryNative", {
  platform: process.platform,
  send(payload) {
    ipcRenderer.send("mory:message", payload);
  },
  request(method, args = {}) {
    return ipcRenderer.invoke("mory:request", { method, args });
  }
});
