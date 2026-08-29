import { mkdirSync } from "node:fs";
import { spawnSync } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const target = process.argv[2] || process.platform;
const architecture = process.argv[3] || process.arch;
const goOS = target === "win32" || target === "windows" ? "windows" : "darwin";
const goArch = architecture === "x64" ? "amd64" : architecture;
const extension = goOS === "windows" ? ".exe" : "";
const output = path.join(root, ".build", "slidev", `mory-slidev${extension}`);
mkdirSync(path.dirname(output), { recursive: true });

const result = spawnSync("go", ["build", "-trimpath", "-o", output, "./cmd/mory-slidev"], {
  cwd: root,
  env: {
    ...process.env,
    GO111MODULE: "auto",
    GOCACHE: path.join(root, ".cache", "go-build"),
    GOOS: goOS,
    GOARCH: goArch
  },
  stdio: "inherit"
});
if (result.error) throw result.error;
if (result.status !== 0) process.exit(result.status || 1);
