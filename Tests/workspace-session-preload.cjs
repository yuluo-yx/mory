const argument = process.argv.find(value => value.startsWith('--mory-session-fixture='));
const fixture = JSON.parse(argument.slice('--mory-session-fixture='.length));
const entries = [
  { id: 'alpha', name: 'Alpha', provider: 'local', localPath: '/workspaces/alpha' },
  { id: 'beta', name: 'Beta', provider: 'local', localPath: '/workspaces/beta' },
  { id: 'missing', name: 'Missing', provider: 'local', localPath: '/workspaces/missing' }
];
window.__sessionEvents = [];
window.moryNative = {
  platform: 'electron',
  send(event) {
    window.__sessionEvents.push(event);
    if (event.type === 'ready') {
      if (!localStorage.getItem('test.seeded')) {
        for (const [key, value] of Object.entries(fixture.storage || {})) localStorage.setItem(key, value);
        localStorage.setItem('test.seeded', 'true');
      }
      window.Mory.initializeSession({ hasWorkspaceRecords: fixture.existing, activeId: '', workspaces: entries });
      if (fixture.openFile) window.Mory.openDocument({ path: '/external/note.md', markdown: '# External note' });
    }
    if (event.type === 'openFile') window.Mory.openDocument({ path: event.path, markdown: `# ${event.path.split('/').at(-1)}` });
  },
  async request(method, args) {
    if (method === 'activateWorkspace') {
      if (args.id === 'missing') throw new Error('Directory not found');
      const state = { activeId: args.id, workspaces: entries };
      window.Mory.setWorkspaceSnapshot({ state, files: [
        { path: `/workspaces/${args.id}/first.md`, name: 'first.md' },
        { path: `/workspaces/${args.id}/last.md`, name: 'last.md' }
      ], directories: [] });
      return state;
    }
    if (method === 'workspaceDocuments' || method === 'customThemes') return [];
    return {};
  }
};
