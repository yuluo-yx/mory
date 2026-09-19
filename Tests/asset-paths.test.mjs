import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import test from 'node:test';
import { rebaseSavedAssetPaths } from '../Sources/Mory/Web/editor-features.js';

const cases = JSON.parse(await fs.readFile(new URL('./fixtures/asset-paths.json', import.meta.url), 'utf8'));
for (const item of cases) {
  test(`asset URLs: ${item.name}`, () => {
    assert.equal(rebaseSavedAssetPaths(item.source, { [item.from + '/']: item.to + '/' }), item.expected);
  });
}
