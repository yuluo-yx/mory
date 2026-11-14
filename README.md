<p align="center"><img src="resources/icon.png" width="96" alt="Mory icon"></p>

# Mory

A local writing app for macOS 13+, built with Go and MyGo native UI. No account or background server is required.

## Features

- Write, source, split and read modes for Markdown and Feishu/Lark DocxXML.
- Rich text, nested documents, tables, LaTeX, images, attachments, outlines and folding.
- Folders, search, favorites, batch operations, multiple windows and quick notes.
- Five document themes, editable CSS, light/dark appearance and English/Chinese UI.
- Import files or folders; export Markdown, DocxXML, self-contained HTML and PDF.
- Static Docusaurus/MDX support: front matter, admonitions, heading IDs, code metadata, details and tabs. JavaScript and React components are preserved as source, not executed.

Use **Help → Open welcome guide** for the editable [syntax and feature guide](resources/welcome.md).

## Everyday use

New documents have an empty title with an **Untitled** placeholder. The sidebar shows a fallback name without storing it as document content.

The library sidebar and document outline collapse independently. Focus mode temporarily hides both without changing their individual visibility preferences. The outline can be placed on either side and hidden from its header. Restore it from the toolbar or **Cmd+Shift+H**. **Settings → Editor** controls text size, page placement, outline appearance and history retention.

In **Manage**, **Select all** switches to **Deselect all** for the current filtered list. **Cancel** clears selections and exits; it does not undo batch actions already performed. Canceling the move dialog keeps the selected documents.

Page width offers **Standard**, **Wide**, **Full**, and **Custom** in document details and **Settings → Editor**. Custom widths range from 480 to 3840 logical pixels, with a draggable slider and numeric control. The preference is saved per document and copied when duplicating it; smaller windows automatically constrain the page to the available space. Existing documents keep their previous width. Split mode continues to fill its available space.

Type a code fence such as \`\`\`go on its own paragraph and press **Enter** to create a code block with its language preserved. The toolbar and **Insert → Code block** use the same code editor. Each block provides a language field, syntax highlighting for supported languages, wrapping, and **Copy code**. An empty language field means plain text. Existing fenced Markdown opens in this editor without rewriting its source just by viewing it.

**Move to Trash** uses Mory's internal Trash, available on the home page beside **My documents / Favorites**. Documents can be restored there. Permanent deletion and emptying Trash require confirmation; external originals and shared assets are kept.

Common shortcuts are **Cmd+N** (new), **Cmd+O** (import), **Cmd+Shift+O** (open original), **Cmd+S** (save), **Cmd+F** (find), **Cmd+J** (search), and **Cmd+,** (settings). Customize them under **Keyboard shortcuts**. The global quick-note shortcut is off until configured.

**Settings → General → Default application** associates `.md`, `.markdown`, and `.mdx` with the running Mory app bundle. The macOS bundle imports explicit Markdown and MDX type declarations so this also works when no other editor has registered those formats. The MDX identifier is a Mory-provided fallback for the external format. Changes to these declarations require rebuilding the `.app`; running a standalone Go binary does not install them.

## Files and recovery

The default library is `~/Documents/Mory`; set `MORY_LIBRARY` to use another directory.

| Path | Contents |
| --- | --- |
| `notes/` | Document source and metadata, one JSON file per document |
| `assets/` | Local images and attachments |
| `history/` | Previous saved versions |
| `recovery/` | Recoverable unsaved content |
| `themes/`, `plugins/` | Custom CSS and local JavaScript transforms |
| `settings.json` | Preferences and shortcuts |

**Import** creates a copy. **Open local file** edits the original with conflict detection. Unchanged documents retain their original source bytes. Local assets are copied on import and included in exports; remote images are not downloaded.

Autosave writes changed content about every 750 ms using atomic file replacement. Failed saves preserve the edit buffer and write a recovery copy when possible. Recovery is offered on the next launch.

**Settings → Editor → Document history** controls snapshots separately from autosave. Defaults: **5 minutes**, **30 days**, **100 versions**, **64 MiB per document**. Old versions are coalesced and pruned when changed content is saved or history is opened. At least the latest version is retained, even if oversized or expired. Disabling history stops ordinary snapshots and pruning without deleting existing copies. Restoring a version first checkpoints the current content. Permanently deleting a document also removes its history and recovery files.

Existing libraries need no manual migration. Back up the library for rollback: replacing the app cannot recover versions already pruned by retention rules.

## Plugins

Place an ES module in the library's `plugins/` directory, then reload plugins:

```js
export function transform({ markdown }) {
  return { markdown: markdown.replace(/TODO:/g, "- [ ]") };
}
```

Transforms receive `{ title, markdown }` and return `{ markdown }`, `{ message }`, or both. They run in an isolated moejs runtime with a 32 MiB allocation limit and a 750 ms deadline. File, network and Node.js APIs are unavailable. Built-ins clean whitespace, count text and insert a table of contents.

## Development

Go is pinned in `mise.toml`; dependency versions are locked in `go.mod` and `go.sum`.

```sh
mise exec -- make run     # Development app
mise exec -- make check   # Tests, vet, native E2E and >=90% combined coverage
mise exec -- make race    # Race detector
mise exec -- make bench   # Microbenchmarks
mise exec -- make build   # Universal macOS app, DMG and checksums
```

Native E2E requires a logged-in macOS desktop and uses isolated libraries. Build outputs are under `build/darwin-universal/`. Local packages are ad-hoc signed; Developer ID distribution and notarization require signing credentials.

For real-window measurements, run `mise exec -- python3 scripts/performance.py`; its fixtures and temporary libraries are removed afterward. Dense documents and very long unbroken lines can make rich editing slow; source mode is preferable for large files. The 32 MiB input limit is not a guarantee of smooth editing at that size.

## License

[MIT](LICENSE). Dependency and artwork notices are in [THIRD_PARTY.md](THIRD_PARTY.md) and included in the app bundle.
