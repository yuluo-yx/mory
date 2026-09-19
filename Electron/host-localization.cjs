const messages = require('../Sources/Mory/Web/host-messages.json');

function hostMessage(message, locale) {
  const original = String(message);
  if (locale === 'en' && messages[original]) return messages[original];
  const text = original.replace(/[。.]$/, '');
  if (locale !== 'en') {
    if (text.toLowerCase() === 'path must remain inside the selected directory') return '路径必须位于所选目录内。';
    return original;
  }
  if (messages[text]) return messages[text];
  const colon = text.indexOf('：');
  if (colon > 0 && messages[text.slice(0, colon)]) {
    return `${messages[text.slice(0, colon)]}: ${hostMessage(text.slice(colon + 1), locale)}`;
  }
  return original;
}

module.exports = { hostMessage };
