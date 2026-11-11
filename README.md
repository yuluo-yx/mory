<p align="center"><img src="resources/icon.png" width="128" alt="Mory illustrated portrait icon"></p>

# Mory

A thoughtful local writing space for macOS, built in **Go** with **MyGo native UI**, **moejs** JavaScript plugins and native **KaTeX** equation rendering.

The library, editors, document previews and settings use native GPU controls. Printing and PDF export use a temporary macOS WebKit document. Quick Look provides system previews for attachments. No account, Node.js, Electron, hosted frontend or background server is required. Online document services are outside the application's scope.

## Writing and organizing

The built-in [welcome guide](resources/welcome.md) is an editable syntax gallery, with Markdown examples, heading levels 1–9, tables and merged cells, equations, offline artwork, Docusaurus callouts/details/tabs, colors, alignment, columns, a countdown, and instructions for attachments, reminders, document references, shortcuts, exports and local JavaScript transforms. Switch between Write, Source and Split to compare the examples with their source. **Help → Open welcome guide** opens the current guide for existing libraries as well. An untouched current copy is reused; edited, older or trashed guides are preserved and a new copy is created.

- Write, Source, Split and Read modes. Split keeps the source editor and rendered preview below the document header and synchronizes scrolling in both directions by matching source blocks to preview paragraphs. Re-entering Split or editing the source refreshes their alignment. Opening or switching an unchanged document preserves its original source bytes.
- Rich paragraphs, nine heading levels, nested lists, tasks, quotations, callouts, code blocks, dividers, links, text colors and highlights.
- Native tables with rectangular selection, merged cells, row/column insertion, deletion and reordering, column resizing and editable cell contents.
- Multiple columns, folding, block drag ordering, block selection, rich copy/paste, outline, find/replace, focus mode and slash insertion.
- In Write mode, hover over a heading to show its level and fold control in the left margin. Folding hides its section up to the next heading of the same or higher level, preserves nested fold states, and leaves the document source unchanged. Outline navigation reveals a target's collapsed parent sections.
- Block and inline LaTeX, including fractions, matrices, integrals and equation numbers. KaTeX executes in moejs; the editor draws its output natively.
- Images with captions, size, alignment and visual cropping; document covers and emoji icons. Local attachments support Quick Look and thumbnail previews.
- Inline reminders, document references, buttons, child-page lists and countdown timers. Reminders are checked while the app is running.
- Cards or a list, full-text search, recent notes, favorites, pinning, folders, nested documents and manual ordering.
- Batch duplication, movement, deletion and restoration. Deleting a parent groups its children for restoration. Emptying Trash requires confirmation.
- Native macOS menus, independent note windows, a quick-note window with an optional global shortcut, and configurable application, editing and formatting shortcuts.

## Import and export

**Import** copies Markdown or Feishu/Lark DocxXML documents. Import a folder to include its subfolders, or an indexed DocxXML library to map metadata, parent relationships and assets. **Open local file** edits the original file in place, with conflict detection. Removing an external file from the library never deletes the original. Duplicating an external note copies its images, attachments and cover into the library.

Export Markdown, DocxXML, self-contained HTML or paginated PDF, or print through the macOS print dialog. Structured HTML preserves table spans, columns, images and mathematical notation. Markdown uses embedded XML for structures that plain Markdown cannot represent. Unknown XML elements and attributes remain in the document model. Cloud-only objects retain their data without connecting to hosted services.

Imports copy local assets. Markdown/XML exports include an `assets` folder; HTML/PDF exports embed local images. Resources must resolve inside the selected import folder. Remote images are not downloaded.

## Docusaurus documents

Mory opens `.md` and `.mdx` with a native, static document preview. Compatibility follows the [document format](https://docusaurus.io/docs/create-doc) and [Markdown features](https://docusaurus.io/docs/markdown-features), with these supported extensions:

- YAML front matter remains byte-for-byte source metadata and is omitted from the body and outline.
- `:::note`, `:::tip`, `:::info`, `:::warning`, `:::danger` and `:::caution` render as callouts, including titles and nested directives. Edits retain the directive syntax.
- Heading IDs such as `## Heading {#stable-id}` are hidden from displayed text, retained on edits and HTML export, and used for native anchor navigation.
- Fenced code retains its metadata, displays `title`, supports `showLineNumbers`, `{1,3-5}` highlighting and standard `highlight-next-line` / `highlight-start` / `highlight-end` comments. Magic comments remain visible and editable in the native code editor.
- `<details><summary>…</summary>…</details>` and static `<Tabs><TabItem value="…" label="…">…</TabItem></Tabs>` support nested Markdown. Details collapse; tabs switch and share selection through `groupId` within a document. Static `defaultValue` and `default` are supported. View choices do not modify source; HTML/PDF exports include every tab.
- Standard Markdown tables, tasks, links, local images and math use Mory's existing document support. Source mode preserves original Markdown and MDX spelling.

This is not a Docusaurus site runtime. React components, JavaScript expressions and imports/exports are retained for source editing, not executed. Computed JSX props (including `values` arrays), site routing, `@site` aliases, live code execution, custom remark/rehype plugins and Mermaid rendering require the original site's build. Mermaid fences remain editable code. Source-only MDX is shown explicitly rather than replaced with invented output. Generic remote JavaScript plugins remain deferred.

## Appearance and shortcuts

Craft, Feishu, Notion, Obsidian and Docusaurus document styles are built in, with System, Light and Dark appearance. Document CSS is separate from the app interface. Settings can edit, duplicate, import and reset styles. Files in the library's `themes/` folder reload after they change. Invalid or unsupported declarations appear in the stylesheet editor.

**General → Default application** registers the running Mory app bundle as the macOS handler for `.md`, `.markdown` and `.mdx`. The operation runs only when the button is clicked and reports success or a system error. Open the installed application before using it; an unpackaged command-line binary cannot become a Finder application. To undo the association, choose another application in Finder's **Get Info → Open with → Change All**.

Settings opens as a full workspace page with four categories: **General** for language and library storage, **Editor** for text size, page width and the document outline, **Themes** for appearance and document styles, and **Keyboard shortcuts** for bindings and shortcut hints. **General → Language** switches the application between English and Simplified Chinese immediately, including app menus and MyGo controls. The choice is saved and shared by open windows. Document contents, titles, folder names and stored theme identifiers remain unchanged. Some macOS-owned system dialog text follows the app language on its next launch.

The document outline has two position icons for **Left** and **Right**. It stays beside the page and starts near the document title, below any cover. Its placement and text size are available under **Editor → Document outline** and persist across launches. Outline text defaults to 14 and can be set from 12 to 22 independently of document text, with a live preview and a reset to the default. Existing libraries without an outline size use the new default without a data migration. **Editor → Page placement** positions the cover, title, body and nearby outline together on the Left, Center or Right of the available writing area. Pages are centered by default; Full width and Split use all available width. Paragraph alignment remains a separate formatting choice. Opening Settings retains the active document and edit buffer; Done or Escape returns to it.

The stylesheet editor provides CSS syntax highlighting, line numbers, bracket matching and undo/redo, with colors that follow the app's light or dark appearance. Selectors include `.page`, `.title`, `h1`–`h9`, `p`, `pre`, `code`, tables, lists, tasks, quotations and callouts. CSS custom properties, `var()` fallbacks, hexadecimal/RGB/HSL colors and relative lengths are supported. Built-in styles in `internal/doctheme/assets/` provide editable examples.

Open **Settings → Keyboard shortcuts** to change bindings. Defaults: **Cmd+N** new note, **Cmd+O** import, **Cmd+Shift+O** open a local file, **Cmd+S** save, **Cmd+F** find, **Cmd+P** insert a block, **Cmd+Alt+P** print, **Cmd+,** settings.

**Cmd+J** focuses library search. The shortcut list includes window commands, clipboard commands, editor modes, formatting and outline placement. Enter modifiers and a key separated by `+`; `Opt` and `Option` are aliases for `Alt`. Changes remain a draft until **Save shortcuts** validates them. Duplicate bindings, including conflicts between local commands and the global quick-note shortcut, are rejected. An unavailable system shortcut reports an error while retaining the previous registration. Menu accelerators and shortcut hints use the saved mappings.

Leave a binding empty to disable it. **Global quick note** is disabled by default, so Mory does not claim Option+Space from launchers such as Raycast. Set an available combination (for example `Ctrl+Alt+Shift+Space`) to enable it, or clear the field and save to release it. Existing explicit global bindings are preserved; an absent or empty legacy `quickNote` value now means disabled instead of implicitly registering Option+Space. No document migration is needed.

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

## Performance

The measurements below are from **2026-11-11**, on an **Apple M4 (10 cores), 24 GiB RAM, macOS 27.0.1**, using an **APFS external USB SSD** and a native window with an effective **2560 × 1312 logical-pixel** content area. The initial matrix inherited macOS's restored maximized window rather than the application's 1280 × 820 default; a follow-up native geometry probe verified the effective size. The checked-in harness explicitly requests the measured size, records actual dimensions, and keeps benchmark window preferences in memory so subsequent runs neither depend on nor overwrite user window settings. These results describe this machine and these fixtures, not a cross-editor comparison or a performance guarantee.

The native benchmark uses an optimized arm64 build (`-trimpath -ldflags="-s -w" -tags=performance,mygo_noinspector`), with neither race nor coverage instrumentation. Each scenario gets a fresh process, a separate library containing the welcome guide, and a generated UTF-8 file. Filesystem caches are **warm** because the fixtures have just been written; this is not a cold-disk or post-reboot test. Autosave and normal garbage collection remain enabled.

### Startup, idle memory and CPU

Across the **39 successful sessions** in the main test matrix:

| Measurement | Result |
| --- | ---: |
| Fresh process launch to captured home frame, median / P95 | 232.0 / 250.8 ms |
| Slowest fresh-process startup in the successful sessions | 651.6 ms |
| Idle home-window resident memory, median | 174.3 MiB RSS |
| Idle process CPU, median | 0.13% of one logical CPU |

RSS comes from `ps`, sampled every 50 ms. Settled memory is the median of samples collected for one second after a two-second pause. Idle CPU is the change in process user + system CPU time during that one-second interval; 100% means one fully occupied logical CPU. These are short idle samples, not energy or battery-life measurements. RSS includes resident shared/native memory and is **not** macOS Activity Monitor's physical-footprint metric. Screenshot buffers used by this test can also contribute to memory use.

### Opening, editing and saving ordinary Markdown

Each row contains **three independent opens**, **30 sequential one-character edits per process** (90 edit samples), and one forced-dirty save per process. Open, save and settled RSS columns show medians; peak RSS is the highest observed sample across the three runs. P95 uses the nearest-rank method. With only three open/save samples, their medians are descriptive, not statistically stable tail estimates.

**Open + capture** includes reading the original file, adding its library record, parsing/editor initialization, selecting the requested mode, rendering, and PNG capture. It excludes the file picker. **Edit + capture** starts before dispatch to the UI thread and ends after a fresh PNG capture, including queueing, application work, layout, drawing and PNG encoding. The adapter invokes the editor's insert API on the UI thread, without synthesizing OS keyboard or IME events. It is not physical keyboard-to-display latency or a display FPS measurement. **Save** times `app.save()` through completed synchronous writes, including pending serialization, revision/recovery work, conflict checks and atomic file replacement; a new edit is inserted immediately beforehand in the same UI callback so autosave cannot turn the measured save into a no-op. Save timing excludes that insertion and the subsequent screenshot.

| Approx. UTF-8 size | Mode | Open + capture (ms) | Edit + capture P50 / P95 (ms) | Save (ms) | Loaded RSS (MiB) | Sampled peak RSS (MiB) |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| 100 KiB | Write | 91.1 | 92.0 / 124.3 | 72.0 | 190.6 | 218.5 |
| 100 KiB | Source | 91.4 | 50.8 / 80.2 | 26.2 | 191.8 | 246.0 |
| 1 MiB | Write | 125.3 | 46.9 / 78.5 | 470.5 | 232.4 | 303.3 |
| 1 MiB | Source | 121.4 | 50.8 / 74.1 | 39.5 | 230.4 | 304.1 |
| 10 MiB | Write | 448.5 | 48.0 / 4,732.6 | 4,564.6 | 980.2 | 1,181.6 |
| 10 MiB | Source | 456.1 | 52.5 / 306.2 | 146.3 | 914.0 | 1,187.2 |

These files contain repeated short paragraphs with bold text, a link, English text and Chinese text. Their exact sizes and line counts are **102,384 bytes / 1,423 lines**, **1,048,464 bytes / 14,563 lines**, and **10,485,648 bytes / 145,635 lines**. Complete paragraphs are repeated rather than cutting a UTF-8 sequence to reach a round byte count.

The 10 MiB Write result exposes an important limitation: ordinary typing can have a low median while full serialization during autosave blocks progress for several seconds. Large rich documents defer serialization to the save boundary; smaller documents can serialize during editing, which also explains why the 100 KiB Write fixture has a higher median than the 1 MiB fixture. Opening speed alone does not establish smooth editing or fast saves. Source mode was substantially cheaper to save in these fixtures, but still reached roughly 1.16 GiB peak RSS at 10 MiB.

### Dense blocks and very long lines

Document structure matters as much as byte count. Dense fixtures repeat a heading, a Docusaurus tip and a table: **102,312 bytes / 8,821 lines** and **1,048,524 bytes / 90,391 lines**. Long-line fixtures contain a single paragraph without newline characters: **102,361 bytes** and **1,048,551 bytes**. The same three-process, 90-edit method applies to successful rows.

| Fixture | Mode | Open + capture (ms) | Edit + capture P50 / P95 (ms) | Save (ms) | Loaded RSS (MiB) | Sampled peak RSS (MiB) |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Dense blocks, 100 KiB | Write | 114.2 | 65.4 / 100.3 | 43.3 | 209.1 | 274.0 |
| Dense blocks, 100 KiB | Source | 115.4 | 53.3 / 78.1 | 29.0 | 206.4 | 285.7 |
| Dense blocks, 1 MiB | Write | 402.4 | 330.8 / 628.9 | 156.4 | 563.8 | 680.5 |
| Dense blocks, 1 MiB | Source | 407.0 | 58.6 / 124.5 | 46.3 | 574.2 | 777.1 |
| Single line, 100 KiB | Write | 86.6 | 453.0 / 818.0 | 25.9 | 190.8 | 251.5 |
| Single line, 100 KiB | Source | 78.8 | 55.9 / 85.3 | 29.2 | 188.7 | 229.1 |
| Single line, 1 MiB | Write | See diagnostic below | **Full scenario timed out, 3/3 runs** | No comparable sample | No comparable sample | No comparable sample |
| Single line, 1 MiB | Source | 97.5 | 57.9 / 87.0 | 42.4 | 189.0 | 250.5 |

The 1 MiB single-line Write case failed to complete the full 30-edit scenario within the per-command 60-second timeout in all three runs and is **not a supported smooth-editing workload**. A separate **one-process, one-edit diagnostic**, excluded from the aggregate tables, completed: opening through capture took **39.7 seconds**; synchronous save work took **49.9 ms**, but save through the next capture took **40.4 seconds**, and undo through capture took **40.6 seconds**. Loaded RSS was **529.4 MiB**, with **625.7 MiB** sampled peak RSS. This separates long-line layout/display cost from the actual write to disk; a successful save does not establish a responsive interface.

A 1 MiB file with normal paragraph breaks is a different workload and should not be used to advertise unlimited long-line support. Dense 1 MiB block documents also show substantial Write-mode latency. The application currently rejects non-UTF-8 documents and files larger than **32 MiB** through its import/open validation; this input limit is not a guarantee of interactive performance up to that size.

### Scrolling and undo

Source scrolling advances the viewport by 400 pixels per operation, with 10 captures per process (30 samples per row). Undo has one sample per process after the edit/save sequence. Both timings include native capture and PNG encoding; no refresh-rate claim is inferred from them.

| Ordinary Markdown in Source mode | Scroll + capture P50 / P95 (ms) | Undo + capture median (ms) |
| --- | ---: | ---: |
| 100 KiB | 50.2 / 51.5 | 50.5 |
| 1 MiB | 51.0 / 51.6 | 49.6 |
| 10 MiB | 52.1 / 55.7 | 103.7 |

### Reproducing and interpreting the results

Run from a logged-in macOS desktop, with no other benchmark or build running concurrently:

```sh
# Full real-window matrix; outputs environment details and individual samples.
mise exec -- python3 scripts/performance.py > performance.jsonl

# Shorter smoke run; not equivalent to the published matrix.
mise exec -- python3 scripts/performance.py --runs 1 --edits 5 --sizes 102400,1048576 --kinds prose

# Internal editor/count/search microbenchmarks, separate from native captures.
mise exec -- make bench
```

`--width` and `--height` select the content dimensions; inspect the actual `window_width` and `window_height` in each command result, especially on smaller displays. A different window size changes the layout workload. `make perf` runs the full native script too. The benchmark adapter is compiled only with the `performance` build tag and is absent from normal releases. Its private Unix socket controls the actual app code paths; it does not transfer the full document through accessibility for each keystroke. Binaries, test libraries, fixtures, histories and sockets live under a temporary `.test-output/` directory and are removed on exit. Failed scenarios are recorded explicitly in JSON rather than replaced with a zero or a fabricated throughput value. The full matrix deliberately includes a slow/unsupported long-line case and can take several minutes.

The separate ten-iteration internal benchmarks on the same date produced:

| Internal operation | Mean time | Go allocations per operation |
| --- | ---: | ---: |
| Editing frame, approximately one million characters | 0.235 ms | 81,685 bytes / 1,306 allocations |
| Unicode word/character counting | 9.260 ms | 0 bytes / 0 allocations |
| Search across 1,000 notes | 2.926 ms | 2,834,164 bytes / 2,014 allocations |

Allocated bytes per operation describe allocation traffic, not retained process memory. The editing benchmark excludes native presentation, PNG capture, autosave serialization and disk writes; its number must not be substituted for the end-to-end columns above.

Not measured here: cold filesystem caches, physical input-to-photon latency, large-image or equation-heavy documents, startup of a large existing library, network volumes, prolonged memory growth, power-loss recovery, Intel hardware, or older macOS releases. The short idle CPU sample is not a battery-use estimate, and the sampled RSS peak is not a guaranteed lifetime maximum.

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

Last local verification: **2026-11-11**, macOS 27.0.1, Apple M4, Go 1.27.3.
`make check` passed at **6,368 / 7,061 production statements (90.19%)**.
`make race`, `go mod verify`, the universal build, strict code-signature verification
and DMG integrity verification passed. The development app was opened for preview;
native E2E used an isolated library. Native E2E includes original-file saves, folder import, covers,
CSS import, Quick Look thumbnails, multiple windows, language switching, shortcut
configuration, outline placement, the welcome-guide menu and offline artwork, Docusaurus theme selection, static details/tabs,
source preservation and the native print renderer. Default-application tests cover
native API binding, unpackaged-app rejection, requested extensions and failure
reporting without changing the developer's or CI runner's system associations.
The interactive printer-selection sheet is replaced by PDF output in E2E.

See **Performance** above for the native-window matrix, memory measurements,
known large-document limits and separate microbenchmark scope.

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
