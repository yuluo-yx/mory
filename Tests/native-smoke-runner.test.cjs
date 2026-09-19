const assert = require('node:assert/strict');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const test = require('node:test');
const runner = path.join(__dirname, '../scripts/run-native-smoke.mjs');

test('native smoke runner preserves success, failure and skip exit codes', () => {
  for (const code of [0, 1, 77]) {
    const result = spawnSync(process.execPath, [runner, '2', process.execPath, '-e', `process.exit(${code})`], { encoding: 'utf8', timeout: 5000 });
    assert.equal(result.status, code, result.stderr);
    if (code === 77) assert.match(result.stderr, /skipped, not passed/);
  }
});

test('native smoke runner terminates an unresponsive process with a failing deadline', () => {
  const result = spawnSync(process.execPath, [runner, '0.2', process.execPath, '-e', 'setInterval(() => {}, 1000)'], { encoding: 'utf8', timeout: 5000 });
  assert.equal(result.status, 124, result.stderr);
  assert.match(result.stderr, /exceeded/);
});

test('native smoke runner rejects invalid commands and reports spawn failures', () => {
  const invalid = spawnSync(process.execPath, [runner, 'invalid'], { encoding: 'utf8', timeout: 5000 });
  assert.equal(invalid.status, 2);
  const missing = spawnSync(process.execPath, [runner, '1', path.join(__dirname, 'missing-native-smoke-executable')], { encoding: 'utf8', timeout: 5000 });
  assert.equal(missing.status, 1);
  assert.match(missing.stderr, /ENOENT/);
});
