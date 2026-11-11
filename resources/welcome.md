# A little room to think

Welcome to **Mory**, your local writing space. This document is a working tour: edit the examples, switch to **Source** to see their spelling, or use **Split** to keep source and preview together.

Your documents stay on this Mac. Open **Help → Open welcome guide** whenever you want a fresh copy of this tour. Your edited guides are kept.

## Start here

- [x] Open your writing space
- [ ] Create a note with **⌘N**
- [ ] Try **Write**, **Source**, **Split**, and **Read**
- [ ] Hover over a heading to see its level and collapse its section
- [ ] Open **Settings → Editor** to adjust text size, outline size, page width, and page placement

Use the outline to move between sections. Place it beside the document on the left or right. Focus mode hides surrounding navigation; full screen and page placement help keep the writing area comfortable.

## Everyday Markdown

Use **bold**, *italic*, ***bold italic***, ~~strikethrough~~, and `inline code`. A [web link](https://github.com/yuluo-yx/mory) stays clickable. You can also [jump to the Docusaurus examples](#docusaurus-examples).

```markdown
**bold**  *italic*  ***bold italic***  ~~strikethrough~~  `inline code`
[A web link](https://github.com/yuluo-yx/mory)
```

Paragraphs are separated by a blank line. A backslash at the end of a line creates a hard break.\
This is the next line in the same paragraph.

### Heading level 3

Use `#` through `######` followed by a space for Markdown heading levels 1–6. This guide already contains levels 1 and 2.

#### Heading level 4

##### Heading level 5

###### Heading level 6

<details><summary>Additional native heading levels</summary>

Mory also supports levels 7–9 through its structured XML vocabulary:

<h7>Heading level 7</h7>
<h8>Heading level 8</h8>
<h9>Heading level 9</h9>

```xml
<h7>Heading level 7</h7>
<h8>Heading level 8</h8>
<h9>Heading level 9</h9>
```

</details>

### Lists and tasks

- An unordered item
- Another item
  - A nested item
  - A second nested item

1. Collect an idea.
2. Give it a useful heading.
3. Turn the next step into a task.

- [ ] An unfinished task — click its checkbox
- [x] A completed task

### Quotes and dividers

> Good writing begins with a calm place and an open page.
>
> A quote can contain more than one paragraph.

The next line is a divider, written as `---`:

---

## Code and technical writing

Fenced code blocks support syntax highlighting, a language selector, wrapping, and copying. Code remains editable in Write mode.

```go title="hello.go" {3} showLineNumbers
package main

func main() {
    println("Hello from Mory")
}
```

The fence metadata above names the file, highlights line 3, and shows line numbers. Docusaurus highlight comments are supported too; they remain visible and editable:

```javascript
// highlight-next-line
const message = 'A small idea worth keeping';
// highlight-start
console.log(message);
console.log('Keep writing.');
// highlight-end
```

### Tables

| Feature | Where to try it | Format |
| :--- | :---: | ---: |
| Inline formatting | This guide | Markdown |
| Table editing | Click a cell | Native |
| Source preservation | Switch modes | Lossless |

Use the table controls to add or delete rows and columns, select cells, merge or split cells, resize columns, and set cell colors. Simple tables export as Markdown; merged cells need embedded XML.

<details><summary>A merged-cell table and its source</summary>

<table><tbody><tr><th colspan="2"><p>A shared heading</p></th></tr><tr><td><p>Left cell</p></td><td background-color="#e8f4ec"><p>A tinted right cell</p></td></tr></tbody></table>

```xml
<table><tbody>
  <tr><th colspan="2"><p>A shared heading</p></th></tr>
  <tr><td><p>Left cell</p></td><td><p>Right cell</p></td></tr>
</tbody></table>
```

</details>

### Equations

Inline math uses dollar signs: $E = mc^2$. Display math uses a pair of double-dollar delimiters:

$$
\int_0^1 x^2\,dx = \frac{1}{3}
$$

$$
\begin{bmatrix}1 & 2 \\ 3 & 4\end{bmatrix}
$$

Choose **Insert → Math** for a display equation or **Inline equation** for an inline object. Select an equation to edit its LaTeX. Fractions, matrices, integrals, and other KaTeX-supported notation render locally.

## Callouts and Docusaurus {#docusaurus-examples}

GitHub-style callouts work with quoted markers:

> [!NOTE]
> This is a note written with `> [!NOTE]`.

:::tip Try a native preview
Choose **Settings → Themes → Docusaurus** to try the documentation style. The same source also works with Craft, Feishu, Notion, and Obsidian.
:::

:::info Keep the source
Docusaurus YAML front matter stays in Source mode without appearing as body text. IDs, titles, tags, slugs, and other fields keep their original spelling.
:::

:::warning A useful boundary
Mory previews static document features. React components, JavaScript expressions, site-specific aliases, and custom build plugins still need the original site's runtime. Mermaid fences remain code.
:::

<details><summary>Other callout types and nested content</summary>

:::note A plain note
Use `:::note`, `:::tip`, `:::info`, `:::warning`, `:::danger`, or `:::caution`.

:::tip Nested tip
Callouts can contain paragraphs, lists, code, and other callouts.
:::

:::

:::danger Example danger callout
This is only a visual example; it does not perform an action.
:::

:::caution Example caution callout
Choose a title that explains the situation.
:::

</details>

### Collapsible details and tabs

Click a summary to expand or collapse its contents. Click a tab below to change the displayed instructions. These choices do not rewrite the document.

<Tabs groupId="guide-platform" defaultValue="mac">
<TabItem value="mac" label="macOS" default>

Press **⌘N** to create a note and **⌘S** to save. Use **⌘,** for Settings.

</TabItem>
<TabItem value="source" label="Source syntax">

```mdx
<details><summary>Read more</summary>

Nested **Markdown** goes here.

</details>

<Tabs defaultValue="first">
<TabItem value="first" label="First">First tab.</TabItem>
<TabItem value="second" label="Second">Second tab.</TabItem>
</Tabs>
```

</TabItem>
</Tabs>

Static `defaultValue`, `default`, and `groupId` are supported. Tabs sharing a group ID share their choice within the document. HTML and PDF exports include every tab.

<details><summary>Front matter and explicit heading IDs</summary>

Put YAML front matter at the very beginning of a document:

```yaml
---
id: writing-guide
title: Writing guide
slug: /writing
sidebar_position: 1
tags: [writing, local]
---
```

Write `## A stable heading {#stable-heading}` to choose its anchor. Link to it with `[Jump](#stable-heading)`. The anchor marker is hidden in the rendered heading.

</details>

## Images, attachments, and document details

![Mory artwork](assets/mory-welcome.png)

The image above is bundled with this guide and works offline. Insert your own image or file with **Insert → Image or attachment…**. Select an image for caption, dimensions, alignment, and cropping. File attachments can use macOS Quick Look previews.

```markdown
![A useful caption](assets/mory-welcome.png)
```

Use **Add icon**, **Add cover**, or **Document actions** to customize the document. Cover placement and body placement are separate from paragraph text alignment. Import copies local assets; opening a local file edits the original file in place.

## Native blocks inside Markdown

Some features need more structure than standard Markdown provides. Mory keeps these blocks as embedded XML in `.md` files, so their attributes survive editing and export. Another Markdown renderer may not display them the same way.

### Colors, underline, and alignment

<p><u>Underlined text</u>, <span text-color="blue">blue text</span>, and <span background-color="light-yellow">highlighted text</span>.</p>

<p align="center">A centered paragraph.</p>

<p align="right">A right-aligned paragraph.</p>

```xml
<p><u>Underline</u> and <span text-color="blue">color</span></p>
<p><span background-color="light-yellow">Highlight</span></p>
<p align="center">Centered text</p>
```

### Columns

<grid><column width-ratio="1"><h3>Collect</h3><p>Keep rough ideas, questions, and references here.</p></column><column width-ratio="1"><h3>Develop</h3><p>Turn the useful pieces into a paragraph or a plan.</p></column></grid>

Use **Insert → Columns** and adjust the column controls. The source looks like this:

```xml
<grid>
  <column width-ratio="1"><p>Left column</p></column>
  <column width-ratio="1"><p>Right column</p></column>
</grid>
```

### Countdown

<countdown duration="300"/>

This five-minute timer starts only when you press **Start / reset**. Use it for a short writing session. **Stop** returns it to its initial duration.

### Reminders, buttons, and document references

Choose **Insert → Reminder**, **Button**, or **Document link** to add an editable inline object. Configure its properties in Mory; choose an existing note when making a document reference. A reminder is checked while Mory is running.

```xml
<p><time date="2030-01-01" time="09:00"/> A reminder</p>
<p><button text="Project website" href="https://github.com/yuluo-yx/mory"/></p>
```

These are source examples, so reading the guide does not schedule a reminder. Use **Insert → Child pages** to show a parent's child notes. Create child notes with the plus button on their parent in the sidebar.

## Editing, organizing, and keeping your work

| Task | How to do it |
| --- | --- |
| Insert a block | Type `/` in a new paragraph, use **+ Insert**, or press **⌘P** |
| Select or move blocks | Use a block's handle and block actions |
| Find and replace | **⌘F** for Find; **⌘⌥F** for Replace |
| Search the library | **⌘J** |
| Change shortcuts | **Settings → Keyboard shortcuts** |
| Adjust the page and outline | **Settings → Editor** |
| Edit a document theme | **Settings → Themes → Edit CSS** |
| Use Mory for Markdown files | **Settings → General → Default application** |
| Import a copy | **File → Import…** or import a folder |
| Edit an original file | **File → Open local file** |
| Export or print | Export Markdown, DocxXML, HTML, or PDF; use **File → Print** |
| Organize notes | Folders, parent notes, favorites, pins, manual ordering, list or grid view |
| Restore work | Trash, previous revisions in document details, and recovery after failed saves |

Autosave runs after editing begins; **⌘S** saves immediately. Large plain documents defer full serialization to the save boundary. Conflicting writes are reported rather than silently overwriting a newer file.

## A small JavaScript workbench

Open **Plugins** to clean trailing whitespace, count words, or generate a table of contents. Local plugins export a `transform` function:

```javascript title="replace-todos.js"
export function transform({ markdown }) {
  return {
    markdown: markdown.replace(/TODO:/g, '- [ ]'),
    message: 'Converted TODO markers',
  };
}
```

Place the file in the library's `plugins/` folder and select **Reload plugins**. Plugins run locally with time and memory limits; file, network, environment, and Node.js APIs are unavailable. Remote integrations are not part of this plugin interface.

You can now replace this guide with your own words—or keep it nearby as a reference.
