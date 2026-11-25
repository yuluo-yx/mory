Mory 0.0.3 improves everyday writing, document navigation, and local storage in the native macOS Markdown editor.

- Store documents as readable Markdown or DocxXML files with embedded metadata, including history, recovery, and trash records.
- Preserve heading numbering and caret positions across edits, with numbering continued from preceding sections.
- Improve empty-block deletion, list exit, indentation, Unicode editing, and undo/redo across Write, Source, Split, and Read modes.
- Insert pasted files atomically and wait for native clipboard results before updating the document.
- Stabilize document and folder navigation, search, batch actions, trash restoration, and shortcut cancellation.
- Keep the caret visible in dark mode, move from the title into the body with Enter, and use Seravek for Latin text alongside the bundled Luo Chinese font.
- Expand native end-to-end coverage for workspace navigation and everyday writing, and update moejs to v0.1.1 and mygo to v0.9.2.

## Downloads

- **arm64**: Apple Silicon Macs (M-series).
- **amd64**: Intel Macs (x86_64).
- **SHA256SUMS**: SHA-256 checksums for both disk images.

Requires macOS 13 or later. Open the matching DMG and drag Mory to Applications. These packages are ad-hoc signed and are not notarized by Apple; macOS may require confirmation in **System Settings → Privacy & Security → Open Anyway** after the first launch attempt.

## Library migration and rollback

Back up the complete document library before opening it in this version. Existing JSON document records are automatically converted to Markdown or DocxXML files, including history, recovery, and trash records. Source text and metadata are preserved; conflicting destination files stop migration rather than being overwritten.

Older releases do not read the new document storage format. To roll back, reinstall the previous application and restore the library backup created before migration. Reinstalling the app alone does not reverse the storage conversion.
