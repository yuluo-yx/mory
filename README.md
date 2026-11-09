<p align="center"><img src="resources/icon.png" width="128" alt="Mory illustrated portrait icon"></p>

# Mory

A thoughtful local writing space for macOS, built in **Go** with **MyGo native UI**, **moejs** JavaScript plugins and native **KaTeX** equation rendering.

The library, editors, document previews and settings use native GPU controls. Printing and PDF export use a temporary macOS WebKit document. Quick Look provides system previews for attachments. No account, Node.js, Electron, hosted frontend or background server is required. Online document services are outside the application's scope.

## Writing and organizing

- Write, Source, Split and Read modes. Opening or switching an unchanged document preserves its original source bytes.
- Rich paragraphs, nine heading levels, nested lists, tasks, quotations, callouts, code blocks, dividers, links, text colors and highlights.
- Native tables with rectangular selection, merged cells, row/column insertion, deletion and reordering, column resizing and editable cell contents.
- Multiple columns, folding, block drag ordering, block selection, rich copy/paste, outline, find/replace, focus mode and slash insertion.
- Block and inline LaTeX, including fractions, matrices, integrals and equation numbers. KaTeX executes in moejs; the editor draws its output natively.
- Images with captions, size, alignment and visual cropping; document covers and emoji icons. Local attachments support Quick Look and thumbnail previews.
- Inline reminders, document references, buttons, child-page lists and countdown timers. Reminders are checked while the app is running.
- Cards or a list, full-text search, recent notes, favorites, pinning, folders, nested documents and manual ordering.
- Batch duplication, movement, deletion and restoration. Deleting a parent groups its children for restoration. Emptying Trash requires confirmation.
- Native macOS menus, independent note windows, an Option+Space quick-note window, and configurable application and formatting shortcuts.

## Import and export

**Import** copies Markdown or Feishu/Lark DocxXML documents. Import a folder to include its subfolders, or an indexed DocxXML library to map metadata, parent relationships and assets. **Open local file** edits the original file in place, with conflict detection. Removing an external file from the library never deletes the original. Duplicating an external note copies its images, attachments and cover into the library.

Export Markdown, DocxXML, self-contained HTML or paginated PDF, or print through the macOS print dialog. Structured HTML preserves table spans, columns, images and mathematical notation. Markdown uses embedded XML for structures that plain Markdown cannot represent. Unknown XML elements and attributes remain in the document model. Cloud-only objects retain their data without connecting to hosted services.

Imports copy local assets. Markdown/XML exports include an `assets` folder; HTML/PDF exports embed local images. Resources must resolve inside the selected import folder. Remote images are not downloaded.

## Appearance and shortcuts

Craft, Feishu, Notion and Obsidian document styles are built in, with System, Light and Dark appearance. Document CSS is separate from the app interface. Settings can edit, duplicate, import and reset styles. Files in the library's `themes/` folder reload after they change. Invalid or unsupported declarations appear in the stylesheet editor.

Selectors include `.page`, `.title`, `h1`–`h9`, `p`, `pre`, `code`, tables, lists, tasks, quotations and callouts. CSS custom properties, `var()` fallbacks, hexadecimal/RGB/HSL colors and relative lengths are supported. Built-in styles in `internal/doctheme/assets/` provide editable examples.

Open **Settings → Keyboard shortcuts** to change bindings. Defaults: **Cmd+N** new note, **Cmd+O** import, **Cmd+Shift+O** open a local file, **Cmd+S** save, **Cmd+F** find, **Cmd+P** insert a block, **Cmd+Alt+P** print, **Cmd+,** settings.

## Local data and recovery

The default library is `~/Documents/Mory`. Set `MORY_LIBRARY=/absolute/path` to choose another folder.

```text
notes/<id>.json       Exact source and document metadata
assets/              Images and attachments
history/<id>/        Previous saved revisions
recovery/            Recoverable unsaved copies
themes/             Custom document styles
plugins/             JavaScript transforms
settings.json        Appearance, folders and key bindings
.write.lock          Cross-process write lock
```

Autosave runs about 750 ms after editing begins. Large plain Markdown documents defer serialization until the save boundary. Writes synchronize a sibling temporary file before atomic replacement. A lock and content hashes detect competing writers. Clean windows reload disk changes; dirty windows retain their edits.

Document details offer previous revisions. A failed save retains edits in memory, writes a recovery copy when storage permits, and prevents navigation or quitting from discarding the buffer. The next launch offers recovery as a new note. Assets remain when notes are deleted because other notes may refer to them.

No in-place migration is required: imports are copies. To roll back an upgrade, close all Mory windows, replace the application bundle, and restore a library backup if needed. External files remain in their original locations.

## JavaScript plugins

Place an ES module in `plugins/`, then select **Reload plugins**. Export `transform({ title, markdown })` and return `{ markdown }`, `{ message }`, or both:

```js
export function transform({ markdown }) {
  return {
    markdown: markdown.replace(/TODO:/g, "- [ ]"),
    message: "Converted TODO markers",
  };
}
```

Built-ins clean trailing whitespace, report reading statistics and insert a table of contents. Each invocation uses a fresh moejs runtime, a 32 MiB allocation limit and a 750 ms deadline including initialization. File, network, environment and Node.js APIs are not installed. Plugins act on Markdown; export XML as Markdown first when a transform is needed.

## Build and verify

Pinned versions: Go **1.27.3**, MyGo **0.8.3**, moejs **0.1.0-alpha.11**, KaTeX **0.16.22**.

```sh
mise exec -- make run
mise exec -- make check
mise exec -- make race
mise exec -- make bench
mise exec -- make build
```

`make check` runs unit, contract and native UI tests, `go vet`, a real macOS window E2E scenario and a **90% combined production statement coverage gate**. E2E file pickers use deterministic fixtures. Automation is excluded from release builds. Race tests cover writers and plugins. Benchmarks cover large-document editing, counting and search; editing-frame results exclude debounced serialization and are not display-latency measurements.

The macOS-only GitHub workflow builds a universal Apple Silicon/Intel app and uploads a DMG and checksums. Outputs are in `build/darwin-universal/`: `Mory.app`, `mory-0.1.0.dmg`, and `SHA256SUMS`. Deployment target: macOS 13+.

Local builds are ad-hoc signed. Developer ID signing, Apple notarization and signed in-app update distribution require the owner's release credentials and update-signing configuration. The native update UI reports availability for the current build. Physical Intel hardware and older macOS releases require separate runtime validation.

Last local verification: **2026-11-09**, macOS 27.0.1, Apple M4, Go 1.27.3.
`make check` passed at **5,093 / 5,637 production statements (90.35%)**.
`make race`, `go mod verify`, the universal build, strict code-signature verification
and DMG integrity verification passed. The packaged application launched with an
isolated library. Native E2E includes original-file saves, folder import, covers,
CSS import, Quick Look thumbnails, multiple windows and the native print renderer.
The interactive printer-selection sheet is replaced by PDF output in E2E.

Ten-iteration regression benchmarks measured **0.234 ms** per million-character
editing frame, **10.294 ms** for Unicode counting and **1.619 ms** for a query over
1,000 notes. These samples are not P95 latency or end-to-end save measurements.
Power-loss injection, long-duration stress and physical Intel testing remain
outside the completed local validation.

## License and artwork

Mory is [MIT licensed](LICENSE). Copyright and dependency notices appear in [THIRD_PARTY.md](THIRD_PARTY.md) and ship inside the application bundle.

The app icon is an original generated blue-and-white line illustration of a graceful young adult woman looking back over her shoulder, with straight shoulder-length hair, delicate cobalt contours, a white blouse and a pale blue rounded border on an opaque white background. Its source is `resources/icon.png`, included under the project's MIT license.

<details><summary>Icon generation prompt</summary>

Use case: logo-brand. Asset type: finished Mory macOS app icon, a single square raster illustration. Draw a fresh original head-and-shoulders portrait of a graceful young adult woman around 22 years old, gentle, youthful, bookish and quietly elegant. Style: refined minimalist editorial LINE ART, smooth confident medium-weight cobalt-blue contour strokes, airy white negative space, a few flat pale-blue accents. Facial features drawn with very few delicate lines, natural small eyes, soft round cheeks, small relaxed closed-mouth smile, no makeup. Hair: simple straight shoulder-length hair with wispy parted bangs, lightly tucked behind one ear, described by a small number of flowing blue outlines and white interior space. Simple white round-neck blouse with a few blue contour lines. Composition: centered large readable portrait inside a clean white rounded-square macOS icon tile with a subtle powder-blue inset border, ample safe margin, head fully visible, balanced head-and-shoulders crop. Strict palette: cobalt blue, pale sky blue and white only. Crisp flat line illustration, elegant and recognizable at 32 pixels. No dense dark filled hair, no curls, no glamorous makeup, no lipstick, no heavy eyelashes, no painterly shading, no gouache texture, no gradients, no photorealism, no anime or chibi proportions, no jewelry, no props, no words, no letters, no watermark, no mockup. Keep the white tile opaque and all pixels outside its smooth rounded corners genuinely transparent, no stray marks. Output one finished square icon.

Final background refinement:

Use case: precise-object-edit. Edit the provided blue and white line-art app icon. Preserve the young adult woman's face, hair, smile, proportions, blue contour strokes, blouse and overall layout exactly. Repair ONLY the background: remove every gray blotch, translucent hole, rough edge, shadow and stray mark. Make the entire square canvas solid opaque pure white, including all four corners. Keep the pale-blue rounded-square outline clean and continuous. The areas inside and outside the outline must be completely uniform flat white with no transparency, texture, shadows or gradients. Retain the tiny pale-blue cheek and neck accents. Deliver a clean finished square blue-and-white line-art icon. No text, no watermark.

Final pose refinement:

Use case: identity-preserve. Asset type: Mory macOS app icon. Edit the supplied blue-and-white line illustration. Change the young adult woman's pose to a graceful SIDEWAYS, LOOKING BACK OVER HER SHOULDER pose. Her torso and shoulders turn away toward the right, showing a three-quarter back view of her blouse and one shoulder in the foreground. She naturally turns her head back toward the viewer over that shoulder; show her face in a clear three-quarter side view, with a delicate visible nose silhouette, a gentle small smile and eyes glancing back at the viewer. Make the sideways torso and backward glance unmistakable, anatomically natural and relaxed. Preserve her youthful adult appearance around age 22, quiet elegance, straight shoulder-length hair with light parted bangs, simple white blouse, minimalist cobalt-blue contour strokes, pale-blue accents and generous white negative space. Hair follows the turn and falls naturally along her back, outlined in a few clean flowing lines with white interiors. Center the complete head-and-shoulders portrait with a safe margin inside the existing pale-blue rounded-square border. Preserve the square opaque white canvas and clean flat background. Strict blue-and-white palette. No heavy shading, gradients, texture, solid dark hair, makeup, jewelry, props, text, watermark or extra characters. Keep the illustration crisp and readable as a small app icon.

</details>

Framework references: [MyGo](https://github.com/egoist/mygo), [moejs](https://github.com/MoeForge/moejs), [KaTeX](https://katex.org/), and the [CSS parser](https://github.com/tdewolff/parse).
