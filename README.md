<p align="center">
  <img src="resources/icon.png" width="96" alt="Mory icon">
</p>

<h1 align="center">Mory</h1>

<p align="center">A calm, local writing space for macOS.</p>

Mory is a native macOS writing app for Markdown, MDX, and Feishu/Lark DocxXML documents. Your library stays on this Mac: no account or background server is required.

## Highlights

- Four editing modes: **Write**, **Source**, **Split**, and **Read**.
- Native editing for headings, lists, tasks, tables, LaTeX, code blocks, callouts, tabs, columns, images, attachments, and document links.
- Static Docusaurus and MDX support, including front matter, admonitions, heading IDs, code metadata, details, and tabs. React components, JavaScript expressions, and site-specific build plugins remain source text and are not executed.
- Local organization with folders, nested documents, favorites, pins, search, manual ordering, batch actions, multiple windows, and quick notes.
- Five document themes, editable CSS, light and dark appearance, and English and Simplified Chinese UI.
- Import files or folders and export Markdown, DocxXML, self-contained HTML, or PDF.
- Local and remote images. Relative local assets are stored beside their document; `http(s)` image URLs are rendered when online and remain URLs in the document source.

The editable feature tour is available from **Help → Open welcome guide**. A new library starts with both English and Chinese welcome documents. The complete source is also available as [the English guide](resources/welcome.md) and [the Chinese guide](resources/welcome.zh-CN.md).

## Get started

Open Mory to create a local library, then use the English or Chinese welcome guide as the starting point. The complete editor tour and syntax reference live in [`resources/welcome.md`](resources/welcome.md) and [`resources/welcome.zh-CN.md`](resources/welcome.zh-CN.md).

## Library and recovery

The default library is `~/Documents/Mory`. Set `MORY_LIBRARY` to use another directory.

| Path | Contents |
| --- | --- |
| `notes/` | One JSON record per Mory document, including its source and metadata |
| `assets/` | Local images and attachments |
| `history/` | Saved document revisions |
| `recovery/` | Recoverable content from failed or interrupted saves |
| `themes/` | Custom document themes and CSS |
| `plugins/` | Local JavaScript transforms |
| `settings.json` | Preferences and keyboard shortcuts |

Local asset files use the SHA-256 digest of their content as the filename prefix, so the document stores a stable relative reference such as `assets/6cbb4a435a30b8abe4bb23b1.png`. A single inserted or imported asset may be up to 32 MiB.

Autosave starts after editing begins and writes changed content about every 750 ms using atomic replacement. If saving fails, Mory keeps the edit buffer and writes recovery content when possible; recovery is offered on the next launch.

**Settings → Editor → Document history** controls revisions independently from autosave. The defaults are a 5-minute checkpoint interval, 30-day retention, 100 versions, and 64 MiB per document. Older versions are coalesced and pruned when changed content is saved or history is opened. Disabling history stops new snapshots and pruning without deleting existing copies. Restoring a revision first checkpoints the current document.

Importing a document copies local assets into the destination library and rewrites their relative paths. Markdown and DocxXML exports copy local assets into an `assets/` directory. HTML and PDF exports embed local images. Remote URLs remain remote and are not fetched for export.

Existing libraries require no migration. Back up the library before maintenance or manual cleanup; replacing the application cannot recover revisions that have already been pruned.

## Plugins

Mory includes plugins for cleaning whitespace, reading statistics, and inserting a table of contents. To add a local plugin, place an ES module ending in `.js` in the library's `plugins/` directory and choose **Reload plugins** in Mory:

```js
export function transform({ title, markdown }) {
  return {
    markdown: markdown.replace(/TODO:/g, "- [ ]"),
    message: `Updated ${title}`,
  };
}
```

A plugin receives `{ title, markdown }` and may return `{ markdown }`, `{ message }`, or both. Plugins run locally in an isolated `moejs` runtime. Each source file is limited to 1 MiB, each run has a 750 ms deadline and 32 MiB memory limit, and file, network, environment, and Node.js APIs are unavailable.

## Performance snapshot

Performance is a core Mory differentiator. The benchmark uses the checked-in 1 MiB Markdown fixtures on an Apple M4 and compares the Write request path with Source mode. Long unbroken lines are guarded to Source mode so they cannot trigger an unbounded rich-editor allocation.

<p align="center">
  <img src="benchmark/performance-overview.svg" alt="Mory Markdown performance overview">
</p>

See the [performance data](benchmark/performance-report.md) and [raw measurements](benchmark/performance-data.jsonl).

## Build from source

Mory requires macOS 13 or later. Go is pinned in [`mise.toml`](mise.toml), and dependency versions are locked in [`go.mod`](go.mod) and [`go.sum`](go.sum). Install `mise`, then run:

```sh
mise exec -- make test    # Unit and package tests
mise exec -- make check   # Tests, vet, native E2E, and combined coverage
mise exec -- make race    # Race detector
mise exec -- make bench   # Benchmarks
mise exec -- make run     # Development app
mise exec -- make build   # Universal macOS app, DMG, and checksum
```

The build output is written to `build/darwin-universal/`. Local packages are ad-hoc signed. Native E2E tests and real-window performance measurements require a logged-in macOS desktop session:

```sh
mise exec -- python3 scripts/performance.py
```

## License

Mory is released under the [MIT License](LICENSE). Dependency and artwork notices are listed in [`THIRD_PARTY.md`](THIRD_PARTY.md) and included in the application bundle.
