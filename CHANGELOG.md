# Changelog

## 0.5.1 - 2026-09-08

### Editor fixes

- Preserve HTML attributes containing URLs during typography optimization instead of leaking internal placeholders into the Markdown document.
- Preserve literal private-use characters and code spans with embedded backticks or line breaks while spacing surrounding prose.
- Keep the current editing mode and original Markdown syntax when optimizing typography. Preserve exact undo history and avoid adding an undo step when no changes are needed.
- Capture document identity and content before saving across macOS, Windows, and Electron. Delayed save completions preserve newer edits, other tabs, and closed-document state.
- Preserve unsaved text when reopening an already edited document. Read original Markdown without normalizing untouched preview content.
- Copy image folders during Save As so the original note retains its images. Reject conflicting asset directories and rebase newer edits and undo history to saved image paths.
- Keep asynchronous image imports attached to their source document through tab changes, closing, edits, and Save As. Preserve successful imports when another image fails and support undo.

### Command line

- Reject export destinations that are directories, symbolic links (including dangling links), or hard links to the source document, even with `--force`.
- Accept `--format=jpg` as an alias for JPEG export and document quoted paths, the `--` separator, and PowerPoint export.
- Cover output collisions, explicit replacement, Chinese and spaced filenames, format normalization, and invalid flags through real Cobra command execution tests.
- Render exports into a temporary file beside the destination and publish only complete, nonempty output. Failed or cancelled rendering preserves existing output; concurrent exports cannot overwrite without `--force`.

### Regression coverage

- Add shared document lifecycle scenarios for Electron and native WebKit, filesystem-backed Electron save scenarios, native save contracts, Windows host tests, and concurrent CLI export tests.
- Run the document lifecycle checks in the local desktop verification and macOS packaging recipes.

### Migration and rollback

No migration is required; replace the application directly. Existing Markdown files and workspace settings remain compatible. CLI users exporting through an output symlink must select a regular destination instead.

To roll back, reinstall 0.5.0 without deleting documents or application settings. Keep copies of documents before using typography optimization in the older release. Published tags must not be moved or reused.

## 0.5.0 - 2026-09-06

### Editor fixes

- Keep Unicode find selections aligned with the original text, refresh matches after edits and document switches, and start backward search at the last occurrence.
- Treat replacement text literally, update the visible preview immediately, and support undo/redo for individual and bulk replacements. Advance beyond inserted search text and localize replacement counts.
- Preserve the original source and saved state when switching between source and preview without editing.
- Parse backtick and tilde code fences by their opening character and length. Shorter markers, mismatched markers, and markers followed by text remain code. Support indented and unfinished fences.
- Preserve nested fence examples and consecutive blank lines when saving rendered code. Use a sufficiently long delimiter and support backticks in code titles.
- Support long fences during live typing and close a block only on a complete matching marker line.
- Exclude fenced code from typography changes, word counts, mind maps, document display titles, graph titles, and backlinks.

### Included changes since 0.4.2

- Add heading section folding and retain document content while collapsing sections.
- Include the accumulated workspace, document, editing, and export fixes already on the main branch.
- Update the shared web runtimes, Electron, Go dependencies, and Wails runtime.

### Packaging

- Consume the complete disk-mount output before selecting the macOS device, preventing premature pipe closure from failing DMG packaging.
- Close the DMG's Finder window before unmounting, retry transient busy-device errors, and retain diagnostics and staging files if unmounting fails.
- Resolve the Windows Wails packaging CLI from the version locked by the application. Abort packaging when a native command fails instead of reusing old artifacts.
- Keep npm, its lock file, macOS, and Windows version metadata consistent with an automated regression check.

### Migration and rollback

No migration is required; replace the application directly. Existing Markdown files and workspace settings remain compatible. A subsequent edit may normalize a code block's delimiter to a safe backtick or tilde fence while preserving its content.

To roll back, reinstall the previous release without deleting workspace files or application settings. Keep a copy of documents containing nested code examples before editing them in an older release, which retains the fixed serialization defects. Published tags must not be moved or reused.
