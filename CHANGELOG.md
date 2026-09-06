# Changelog

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

- Resolve the Windows Wails packaging CLI from the version locked by the application. Abort packaging when a native command fails instead of reusing old artifacts.
- Keep npm, its lock file, macOS, and Windows version metadata consistent with an automated regression check.

### Migration and rollback

No migration is required; replace the application directly. Existing Markdown files and workspace settings remain compatible. A subsequent edit may normalize a code block's delimiter to a safe backtick or tilde fence while preserving its content.

To roll back, reinstall the previous release without deleting workspace files or application settings. Keep a copy of documents containing nested code examples before editing them in an older release, which retains the fixed serialization defects. Published tags must not be moved or reused.
