import { spawn } from 'node:child_process';

const [seconds, executable, ...args] = process.argv.slice(2);
const timeout = Number(seconds) * 1000;
if (!executable || !Number.isFinite(timeout) || timeout <= 0) {
  console.error('Usage: run-native-smoke.mjs <timeout-seconds> <executable> [args...]');
  process.exit(2);
}
const child = spawn(executable, args, { stdio: 'inherit' });
let expired = false;
let forceKill;
const timer = setTimeout(() => {
  expired = true;
  console.error(`Native smoke test exceeded ${seconds} seconds.`);
  child.kill('SIGTERM');
  forceKill = setTimeout(() => child.kill('SIGKILL'), 1000);
}, timeout);
child.on('error', error => {
  clearTimeout(timer);
  console.error(error.message);
  process.exitCode = 1;
});
child.on('close', code => {
  clearTimeout(timer);
  clearTimeout(forceKill);
  if (code === 77) console.error('Native smoke test was skipped, not passed.');
  process.exitCode = expired ? 124 : Number.isInteger(code) && code >= 0 ? code : 1;
});
