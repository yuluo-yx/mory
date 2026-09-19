const fs = require('node:fs');
const path = require('node:path');

function isWithin(root, candidate) {
  const relative = path.relative(root, candidate);
  return relative !== '..' && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative);
}

// Resolve the nearest existing ancestor so a new child cannot bypass a linked parent.
// lstat distinguishes a missing destination from a dangling or cyclic link, which fails closed.
function realDestination(value) {
  let ancestor = path.resolve(value);
  const missing = [];
  for (;;) {
    try { fs.lstatSync(ancestor); }
    catch (error) {
      if (error.code !== 'ENOENT' || path.dirname(ancestor) === ancestor) throw error;
      missing.unshift(path.basename(ancestor));
      ancestor = path.dirname(ancestor);
      continue;
    }
    return path.join(fs.realpathSync(ancestor), ...missing);
  }
}

function containedPath(root, candidate) {
  const absoluteRoot = path.resolve(root);
  const absolute = path.resolve(candidate);
  if (!isWithin(absoluteRoot, absolute) || !isWithin(realDestination(absoluteRoot), realDestination(absolute))) {
    const error = new Error('Path must remain inside the selected directory.');
    error.code = 'MORY_PATH_OUTSIDE';
    throw error;
  }
  return absolute;
}

function validateContainedTree(root, candidate) {
  containedPath(root, candidate);
  let info;
  try { info = fs.lstatSync(candidate); }
  catch (error) { if (error.code === 'ENOENT') return; throw error; }
  if (info.isDirectory()) {
    for (const name of fs.readdirSync(candidate)) validateContainedTree(root, path.join(candidate, name));
  }
}

module.exports = { containedPath, realDestination, validateContainedTree };
