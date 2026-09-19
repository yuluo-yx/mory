const filesystem = require('node:fs/promises');
const { randomUUID } = require('node:crypto');
const path = require('node:path');

async function writeAtomicFile(filename, data, fs = filesystem) {
  let destination = path.resolve(filename);
  let mode;
  try {
    destination = await fs.realpath(destination);
    const info = await fs.stat(destination);
    if (!info.isFile()) throw new Error('Replacement target is not a regular file');
    await fs.access(destination, 2);
    mode = info.mode & 0o777;
  } catch (error) {
    if (error.code !== 'ENOENT') throw error;
    // A dangling link must not be silently replaced by a regular document.
    if (await fs.lstat(destination).then(() => true, error => { if (error.code === 'ENOENT') return false; throw error; })) throw error;
  }
  const temporary = path.join(path.dirname(destination), `.mory-write-${randomUUID()}.tmp`);
  let file = await fs.open(temporary, 'wx', mode ?? 0o666);
  try {
    await file.writeFile(data, 'utf8');
    if (mode !== undefined) await file.chmod(mode);
    await file.sync();
    const closing = file;
    file = null;
    await closing.close();
    await fs.rename(temporary, destination);
  } finally {
    try { if (file) await file.close(); }
    finally { await fs.unlink(temporary).catch(error => { if (error.code !== 'ENOENT') throw error; }); }
  }
}

module.exports = { writeAtomicFile };
