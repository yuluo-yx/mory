<p align="center">
  <img src="resources/icon.png" width="96" alt="Mory icon">
</p>

<h1 align="center">Mory</h1>

<p align="center">A calm, local writing space for macOS.</p>

<p align="center"><a href="README.zh-CN.md">简体中文</a></p>

Mory is a native macOS writing app for Markdown, MDX, and Feishu/Lark DocxXML documents. Your library stays on this Mac: no account or background server is required.

![Mory showing a document with tasks, highlighted Go code, a callout, a table, and an outline](resources/screenshots/mory-overview.png)

<details>
<summary>Watch Mory switch between Write, Source, Split, and Read</summary>

![Mory's four document modes in the native macOS app](resources/screenshots/mory-modes.gif)

</details>

## Highlights

- Four editing modes: **Write**, **Source**, **Split**, and **Read**.
- Native editing for headings, lists, tasks, tables, LaTeX, code blocks, callouts, tabs, columns, images, attachments, and document links.
- Static Docusaurus and MDX support, including front matter, admonitions, heading IDs, code metadata, details, and tabs. React components, JavaScript expressions, and site-specific build plugins remain source text and are not executed.
- Local organization with folders, nested documents, favorites, pins, search, manual ordering, batch actions, multiple windows, and quick notes.
- Drag the sidebar's right edge to resize it, or double-click to restore its default width. The width is saved across launches and shared with other windows; narrow windows temporarily limit it to leave room for the document. A focused divider also supports the Left and Right arrow keys.
- On launch, sidebar folders start collapsed except the path to the most recently opened or edited document. Parent documents are revealed for nested notes; unrelated folders stay closed. Opening a document reveals its path without collapsing folders opened manually during that session. Libraries without previously opened documents start with every folder collapsed. Startup keeps the home view and does not rewrite document contents.
- Delete a folder from its context or more menu to move the whole folder into Trash for 30 days, including empty subfolders, documents, and external file references. Restore its folder entry to recover the original paths and document hierarchy together. External originals stay on disk, including after permanently deleting the group. The folder editor also supports removing a folder while keeping its notes. Older deletions without a folder entry retain their existing per-document restore behavior.
- One built-in **yuluo** document theme, custom CSS, light and dark appearance, and English and Simplified Chinese UI.
- Import files or folders and export Markdown, DocxXML, self-contained HTML, or PDF.
- Local and remote images. Relative local assets are stored beside their document; `http(s)` image URLs are rendered when online and remain URLs in the document source.

The editable feature tour is available from **Help → Open welcome guide**. A new library starts with both English and Chinese welcome documents. The complete source is also available as [the English guide](resources/welcome.md) and [the Chinese guide](resources/welcome.zh-CN.md).

Press Enter in a document's title to move the caret to the start of its body in Write, Source, or Split mode. Existing body text is preserved.

The document header shows its library location, including folders, parent documents and the saved filename (for example `/test/test1/test.md`). External files show their absolute path. Long paths stay on one line with a middle ellipsis; hover to see the full path. Mode switches and document actions remain aligned to the window's right edge, including full screen and when the sidebar is hidden.

Press **Cmd+/** to cycle **Write → Source → Read → Split → Write** while preserving pending edits. **View → Cycle editor mode** runs the same action; its key can be changed in shortcut settings. Shortcut help remains available from **Help → Shortcuts**. Older saved Cmd+/ help defaults migrate to mode cycling when no explicit cycle binding exists; custom help bindings remain unchanged.

Enter in a single-line form submits its primary action: folder creation/editing, note rename/move, links, block properties, theme names, and shortcut settings. Find advances to the next match; Replace replaces the selected match and advances. Existing validation also applies to keyboard submissions. IME candidate confirmation does not submit, and multiline equation and stylesheet editors retain Enter for newlines.

Sidebar rows keep a fixed height and reserve space for their actions, so hovering does not move surrounding rows or change title truncation. Actions appear on hover, keyboard focus, or selection; hover over a title to see its full name. Sidebar labels and settings controls use 15-point text, independently of document and outline font preferences.

Settings replaces the document sidebar with five categories: **General**, **Editor**, **Themes**, **Keyboard shortcuts**, and **About**. Select a category or use Up/Down, Home/End on the focused navigation tab. **Back to workspace** or Escape returns to the same document or library view, preserving the editor mode and collapsed-sidebar preference. About introduces Mory and shows the installed version and project link. Settings navigation uses the existing resizable sidebar; no storage migration is needed.

**Settings → Themes** uses **yuluo** as the single built-in document style. **Edit CSS**, **Duplicate theme**, and **Import CSS…** provide custom styles; a style picker appears when custom CSS files are available. Former Craft, Feishu, Notion, Obsidian and Docusaurus selections resolve to yuluo unless a same-named custom CSS file exists. The next normal preferences save persists the resolved selection; documents and custom files are not rewritten. **Reset theme** selects the bundled yuluo style and keeps custom files. An edited `yuluo.css` is preserved as the next available `yuluo-custom-N.css` before resetting. Docusaurus/MDX syntax support remains independent of the document style.

Document scrollbars use a slim rounded thumb in the page's right whitespace, leaving line endings, caret placement, and links clear of the scroll track. Write, Read, and split previews keep this separation; structured blocks use the outer document scrollbar instead of drawing a second one over their text. The text width and title alignment remain stable when the scrollbar appears.

Document context and more menus offer **Copy absolute path** and **Show in Finder**. These use the actual saved Markdown/XML file, or the original file for an externally opened document. Copying a path preserves pending edits and also works when the file is missing; Finder reports a missing file without recreating it. Sidebar folders are logical groups, so **Open storage folder in Finder** opens the library's shared `notes/` directory, including for empty or imported groups. Group names do not imply physical subdirectories. These actions do not change the storage layout.

## Editing tables

Type `/` on an empty paragraph, choose **Table**, and select a size in the 9 × 9 picker. Arrow keys adjust the size and Enter confirms it. Insertion removes the slash, focuses the first cell, and preserves neighboring paragraphs. Cancel leaves the document intact. Wide tables scroll horizontally inside the document without widening the page.

- **Tab / Shift+Tab** move between cells. Tab from the final cell adds a row; Enter creates another paragraph within the cell; **Cmd+Enter** moves to the paragraph after the table.
- Select text to format only those words using the toolbar or keyboard shortcuts. The text toolbar appears for a cell selection even when the persistent formatting toolbar is hidden.
- The table toolbar and dimensions appear while hovering over the table. Row numbers and column letters stay hidden. Hover over the upper or left edge for a selection grip; Shift-click extends the selection. Cell context menus also select rows and columns. The selection toolbar applies bold, italic, underline, strike, inline code, paragraph styles, text color, highlight, cell background and horizontal/vertical alignment to the selected cells together.
- Hover over a column boundary on the table's top edge or a row boundary on its left edge to reveal a **+**. Click to insert at that boundary, including before the first or after the last row/column, and start typing immediately. Hover controls keep their space so showing them never shifts the document.
- **Table actions** and cell context menus insert, delete or move rows and columns, merge/split cells, toggle the header row, open properties, or delete the table. Backspace/Delete on a selected range clears its contents while retaining the grid. At an empty cell boundary these keys preserve the table structure.
- Drag a column boundary within the header row to resize it. Copy/Cut/Paste support tab-separated cell grids, including quoted multiline values. Paste grows the table when necessary, up to 1,000 rows and 100 columns; split merged destination cells before pasting a grid. Structural edits and grid paste each form one undo step.

Large simple tables retain a bounded row viewport with pinned headers. Choose **Edit cell** to materialize a compact Markdown table, then edit normally or select a row/column for formatting. Entering editing alone preserves the original source and does not mark the document dirty. Unchanged tables retain their original Markdown; edited tables use Mory's existing XML table representation when needed to preserve widths, formatting and structure. No storage migration is required for these table changes.

## Get started

Download **Mory 0.0.3** for your Mac:

| Mac | Installer |
| --- | --- |
| Apple Silicon (M-series) | [macOS arm64](https://github.com/yuluo-yx/mory/releases/download/v0.0.3/mory-0.0.3-macos-arm64.dmg) |
| Intel (x86_64) | [macOS amd64](https://github.com/yuluo-yx/mory/releases/download/v0.0.3/mory-0.0.3-macos-amd64.dmg) |

[Release notes](https://github.com/yuluo-yx/mory/releases/tag/v0.0.3) · [SHA-256 checksums](https://github.com/yuluo-yx/mory/releases/download/v0.0.3/SHA256SUMS). Open the DMG and drag Mory to Applications. Packages are ad-hoc signed and not notarized by Apple; if macOS blocks the first launch, confirm **Open Anyway** in **System Settings → Privacy & Security**.

Open Mory to create a local library, then use the English or Chinese welcome guide as the starting point. The complete editor tour and syntax reference live in [`resources/welcome.md`](resources/welcome.md) and [`resources/welcome.zh-CN.md`](resources/welcome.zh-CN.md).

Mory bundles [Luo Regular](https://github.com/tw93/Luo) for Chinese text in its native interface and built-in writing styles. The font works offline without a system installation. English letters and digits use the system-provided Seravek font, while code retains its monospaced font. Seravek is not bundled; unavailable glyphs and emoji use macOS fallback fonts. Custom CSS keeps its authored font cascade; use `font-family: "Seravek", "Luo", sans-serif` to opt in. Native macOS menus and HTML/PDF export retain their existing typography. Font provenance and the SIL OFL 1.1 notice are recorded in [`THIRD_PARTY.md`](THIRD_PARTY.md).

## Library and recovery

The default library is `~/Documents/Mory`. Set `MORY_LIBRARY` to use another directory.

| Path | Contents |
| --- | --- |
| `notes/` | One Markdown or DocxXML file per Mory document, with embedded metadata |
| `assets/` | Local images and attachments |
| `history/` | Saved document revisions |
| `recovery/` | Recoverable content from failed or interrupted saves |
| `themes/` | Custom document themes and CSS |
| `plugins/` | Local JavaScript transforms |
| `settings.json` | Preferences and keyboard shortcuts |

Document filenames are readable in Finder. Imports retain the original basename and extension (including `.mdx`, `.markdown`, and extension casing), even when an XML document has a different display title. New notes use their title, and renaming a library note updates its filename. Duplicate names receive ` (2)`, ` (3)`, and so on; the check includes Trash, case variants, and Unicode normalization variants. External originals keep their own filenames. Internal IDs remain in metadata for references and history.

On first opening an older library, ID-based filenames migrate to readable names without changing source text, timestamps, folder relationships, or document IDs. Older imports did not retain the original filename, so migration uses the current title; the exact former name and extension cannot always be recovered. Existing named files and interrupted migrations are handled without overwriting another document. Back up the library before upgrading: older releases require their original ID filenames, so rollback requires restoring that backup along with the old application.

Local asset files use the SHA-256 digest of their content as the filename prefix, so the document stores a stable relative reference such as `assets/6cbb4a435a30b8abe4bb23b1.png`. A single inserted or imported asset may be up to 32 MiB.

Autosave starts after editing begins and writes changed content about every 750 ms using atomic replacement. If saving fails, Mory keeps the edit buffer and writes recovery content when possible; recovery is offered on the next launch.

After a save failure or a missing external file, background retries pause so dismissing the error stays effective. Use **Retry** to try again, or **Close document** to return to the library. Closing with unsaved edits first writes a recovery snapshot and immediately offers **Recover as a copy**; if recovery fails, the document stays open. Missing external files without pending edits can also be removed from the list without recreating or deleting the original file.

**Settings → Editor → Document history** controls revisions independently from autosave. The defaults are a 5-minute checkpoint interval, 30-day retention, 100 versions, and 64 MiB per document. Older versions are coalesced and pruned when changed content is saved or history is opened. Disabling history stops new snapshots and pruning without deleting existing copies. Restoring a revision first checkpoints the current document.

Importing a document copies local assets into the destination library and rewrites their relative paths. Markdown and DocxXML exports copy local assets into an `assets/` directory. HTML and PDF exports embed local images. Remote URLs remain remote and are not fetched for export.

Back up the complete library before upgrading. Existing JSON document records, including history, recovery, and trash, are automatically converted to Markdown or DocxXML files while preserving source text and metadata. Conflicting destination files stop migration rather than being overwritten. To return to an older release, reinstall that application and restore the library backup created before migration; replacing the application alone does not reverse the conversion or recover pruned revisions.

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

Mory's performance strengths are **low input-processing cost** and **complete large-table reading with bounded rendering work**. On an Apple M4 with 24 GiB RAM, the recorded 1 MiB structured and mixed-format documents have median edit actions below **0.05 ms**. Full-table navigation, including rendering and PNG capture, takes about **35–37 ms** across the three table fixtures.

<p align="center">
  <img src="benchmark/performance-overview.svg" alt="Mory Markdown performance overview">
</p>

| Write workload | Edit action | Edit + rendering + PNG capture | Save, including serialization |
| --- | ---: | ---: | ---: |
| Structured document, 1 MiB | **0.044 ms** | 55.88 ms | 88.56 ms |
| Mixed-format document, 1 MiB | **0.043 ms** | 53.53 ms | 61.99 ms |

Large-table reading covers **401–3,206 rows**, with loaded RSS of about **201–238 MiB**. Every row remains accessible, with pinned headers and wrapped cells. The editing matrix contains 36 independent runs and 1,080 edits; the table-reading matrix contains 18 runs and 162 verified row jumps. Values are medians across three runs. Input actions exclude rendering; capture includes PNG encoding, so neither is keyboard-to-display latency.

Reference editors: **ArcMD v0.0.11** and **Quiet.md v0.1.11**. Mory's measured strengths and the comparison boundaries are documented in the [performance comparison](benchmark/performance-comparison.md); different measurement workflows are not expressed as speed ratios. See the [editing data](benchmark/performance-report.md) and [full-table reading data](benchmark/large-table-performance-report.md) for reproducible results and raw samples.

## Build from source

Mory requires macOS 13 or later. Go is pinned in [`mise.toml`](mise.toml), and dependency versions are locked in [`go.mod`](go.mod) and [`go.sum`](go.sum). Install `mise`, then run:

```sh
mise exec -- make test    # Unit and package tests
mise exec -- make check   # Tests, vet, native E2E, and combined coverage
mise exec -- make race    # Race detector
mise exec -- make bench   # Benchmarks
mise exec -- make run     # Development app
mise exec -- make build ARCH=arm64  # Apple Silicon app, DMG, and checksum
mise exec -- make build ARCH=amd64  # Intel app, DMG, and checksum
```

Build output is written to `build/darwin-arm64/` or `build/darwin-amd64/`; omitting `ARCH` selects the current Go architecture. GitHub Actions builds and tests both architectures on matching macOS runners, then publishes tagged releases with both DMGs and checksums. Local packages are ad-hoc signed. Native E2E tests and real-window performance measurements require a logged-in macOS desktop session.

`make check` includes native workspace and everyday writing journeys. The workspace suite covers unsaved document switching, folder navigation and rename, search, shortcut-draft cancellation, settings navigation, custom CSS editing and reset, hover geometry, canceled batch moves, parent/child trash restoration, and a language round trip. The writing suite covers Unicode selection replacement, composed emoji deletion, formatting boundaries, mode switches, atomic replace-all undo/redo, settings interruptions, empty-list exit, and source editing. Assertions check actual input focus, rendered text and persisted records; fixtures use a temporary library and are removed when the run ends.

Empty quotes, code blocks, callouts, list items, and tasks support direct Backspace and Delete from their focused editor: one key exits the empty structure into a writable paragraph. Deleting selected text leaves its block in place until a subsequent deletion. Native E2E covers both keys, single-step undo/redo, continued typing without refocusing, saved/reopened blocks after lists, and read-only protection; unit/UI tests also protect nested containers, attachments, titles, captions, and table cells.

Document links use a distinct color and an underline. Hold Ctrl and hover over a link to open it once per hover; ordinary hover leaves the document in place. Writing, reading, and split previews share this behavior, including links to other Mory notes. Tests cover pointer modifiers, blank space beside links, wrapped text, selection preservation, theme colors, and unchanged source after navigation.

To run one group while debugging:

```sh
mise exec -- go build -tags=e2e -o .test-output/mory-e2e .
mise exec -- python3 scripts/e2e.py .test-output/mory-e2e --suite workspace
mise exec -- python3 scripts/e2e.py .test-output/mory-e2e --suite writing
mise exec -- python3 scripts/e2e.py .test-output/mory-e2e --suite editor
```

These Unicode tests send committed text and native key events. They do not automate a macOS input method's candidate window or marked-text composition. Real-window performance measurements remain separate:

```sh
mise exec -- python3 scripts/performance.py
```

## License

Mory is released under the [MIT License](LICENSE). Dependency and artwork notices are listed in [`THIRD_PARTY.md`](THIRD_PARTY.md) and included in the application bundle.
