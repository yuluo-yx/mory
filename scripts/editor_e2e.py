"""Editing regressions driven through native pointer and keyboard events.

Accessibility locates targets and reads results; it must not repair focus after
an insertion, deletion, undo or boundary transition under test.
"""
from library_records import note_paths, read_note
import time


def exercise_editor(call, nodes, act, key, root, fixtures, native_space):
    completed = []

    def wait(description, predicate):
        deadline = time.monotonic() + 5
        while True:
            tree = nodes()
            result = predicate(tree)
            if result:
                return result
            if time.monotonic() >= deadline:
                snapshot = [(n.get("label"), n.get("value"), n.get("states")) for n in tree]
                raise AssertionError(f"{description}; tree={snapshot}")
            time.sleep(0.05)

    def field(value=None, label=None):
        return wait(f"Missing editor {label or value!r}", lambda tree: next((n for n in tree
                    if (n.get("label", "").startswith("Block text ") or n.get("label") in {"Code editor", "Document editor"})
                    and (label is None or n.get("label") == label)
                    and (value is None or n.get("value", "") == value)), None))

    def control(label):
        return wait(f"Missing control {label}", lambda tree: next((n for n in tree if n.get("label") == label), None))

    def pointer(node, action="click", **extra):
        box = node["bounds"]
        assert box["width"] > 0 and box["height"] > 0, node
        call("input", events=[dict(type=action, x=box["x"]+box["width"]/2,
                                   y=box["y"]+min(12, box["height"]/2), **extra)])

    def type_text(text):
        call("input", events=[dict(type="text", text=text)])

    def focused(node):
        return call("tree").get("focus") == node["id"]

    def focused_field(value=None, label=None):
        target = field(value, label)
        return wait("Editor did not receive focus: " + target["label"], lambda tree: next((n for n in tree
                    if n.get("label") == target["label"] and focused(n)), None))

    def saved(title):
        call("menu", path=["File", "Save"])
        nodes()
        return next(read_note(p) for p in note_paths(root / "library" / "notes")
                    if read_note(p)["title"] == title)

    def reopen(title):
        pointer(control("Back to library"))
        act("All notes")
        act("Note " + title)

    def new_note(title):
        call("menu", path=["File", "New note"])
        act("Document title", "setValue", text=title)
        pointer(field(label="Document editor"))

    def imported(title, source):
        # Import exercises the production parser and view selection. Restore
        # the shared fixture so subsequent import/export scenarios are isolated.
        path = fixtures / "Fixture import.md"
        previous = path.read_bytes()
        try:
            path.write_text(source)
            call("menu", path=["File", "Import…"])
            wait("Imported document did not open", lambda tree: any(n.get("label") == "Document title"
                 and n.get("value") == "Fixture import" for n in tree))
            act("Document title", "setValue", text=title)
        finally:
            path.write_bytes(previous)

    # A lone dash below a paragraph is ambiguous while a list is being typed.
    # Check every intermediate state, not only the completed list's outline.
    title = "Editing source list outline"
    new_note(title)
    pointer(control("Source"))
    pointer(control("Source editor"))
    type_text("## Stable heading\n\nParagraph\n")
    for text in ("-", " ", "List item"):
        type_text(text)
        tree = nodes()
        headings = {n["label"] for n in tree if n.get("label", "").startswith("Heading ")}
        assert headings == {"Heading Stable heading"}, headings
    note = saved(title)
    assert note["source"] == "## Stable heading\n\nParagraph\n- List item", note
    completed.append("source list prefix / stable outline / unchanged Markdown / save")

    # Enter at a heading's start adds body space, without shifting its outline.
    title = "Editing heading leading Enter"
    imported(title, "## 1. Section\n\n<p>Body</p>\n\n## 2. Next\n\n" + "More text.\n\n" * 40)
    heading = field("1. Section")
    pointer(heading)
    key("Ctrl+Home")
    before_y = heading["bounds"]["y"]
    outline_y = control("Heading 1. Section")["bounds"]["y"]
    for _ in range(3):
        key("Enter")
        heading = focused_field("1. Section")
        assert heading["bounds"]["y"] > before_y + 20, heading
        before_y = heading["bounds"]["y"]
        assert control("Heading 1. Section")["bounds"]["y"] == outline_y
        assert {n.get("label") for n in nodes() if n.get("label", "").startswith("Heading ")} == {
            "Heading 1. Section", "Heading 2. Next"}
    for _ in range(3):
        key("Cmd+Z")
    for _ in range(3):
        key("Cmd+Shift+Z")
    assert saved(title)["source"].count("<p/>") == 3
    reopen(title)
    field("1. Section")
    assert saved(title)["source"].count("<p/>") == 3
    completed.append("heading leading Enter / body position / stable outline / undo / redo / reopen")

    # Previously empty headings left invisible raw markers that blocked joins.
    # Delete through both old headings and saved empty paragraphs at a list edge.
    title = "Editing blank blocks before list"
    imported(title, "<p/>\n\n##\n\n<p/>\n\n##\n\n1. Keep item\n2. Second item\n\n## Next\n\nBody")
    original = saved(title)["source"]
    pointer(field("Keep item"))
    key("Ctrl+Home")
    previous_y = field("Keep item")["bounds"]["y"]
    for _ in range(4):
        key("Backspace")
        item = focused_field("Keep item")
        assert item["bounds"]["y"] < previous_y, item
        previous_y = item["bounds"]["y"]
        field("Second item")
    clean = saved(title)["source"]
    assert "<p/>" not in clean and clean.count("##") == 1, clean
    for _ in range(4):
        key("Cmd+Z")
    assert saved(title)["source"] == original
    for _ in range(4):
        key("Cmd+Shift+Z")
    assert saved(title)["source"] == clean
    reopen(title)
    assert saved(title)["source"] == clean
    completed.append("blank headings and paragraphs / list boundary deletion / geometry / undo / redo / reopen")

    title = "Editing blank before heading"
    imported(title, "<p/>\n\n## Section\n\nBody")
    pointer(field("Section"))
    call("input", events=[dict(type="key", key="Ctrl+Home"), dict(type="key", key="Backspace")])
    focused_field("Section")
    control("Heading Section")
    source = saved(title)["source"]
    assert "<p/>" not in source and "## Section" in source, source
    completed.append("blank deletion preserves heading style and outline")

    # Tab must keep native focus in the body reached by Enter from the title.
    for structured in (False, True):
        title = "Editing body indentation " + str(structured)
        imported(title, "<p>Body</p>" if structured else "Body")
        pointer(control("Document title"))
        key("Enter")
        focused_field("Body")
        key("Tab")
        focused_field("Body")
        key("Cmd+Z")
        key("Cmd+Shift+Z")
        type_text("Indented ")
        focused_field("Indented Body")
        note = saved(title)
        assert note["title"] == title and 'indent="1"' in note["source"], note
        reopen(title)
        pointer(field("Indented Body"))
        key("Shift+Tab")
        key("Shift+Tab")
        focused_field("Indented Body")
        note = saved(title)
        assert "indent=" not in note["source"] and "Indented Body" in note["source"], note
        completed.append("body Tab / focus / undo / redo / save / reopen / outdent: " + str(structured))

    # Numeric heading text must survive the same keystrokes that create lists
    # in ordinary body paragraphs, including a burst of native input events.
    for structured in (False, True):
        for prefix in ("1.", "12)"):
            title = "Editing native heading space " + str(structured) + " " + prefix
            source = "## " + prefix
            if structured:
                source += "\n\n| A | B |\n| --- | --- |\n| one | two |\n"
            imported(title, source)
            pointer(field(prefix))
            key("Cmd+End")
            native_space()
            field(prefix + " ")
            source = saved(title)["source"]
            # Block serialization can trim an otherwise empty heading's trailing space.
            assert source.replace("\\.", ".").replace("\\)", ")").startswith("## " + prefix), source
            key("Cmd+Z")
            field(prefix)
            key("Cmd+Shift+Z")
            field(prefix + " ")
            key("Cmd+End")
            type_text("Overview")
            field(prefix + " Overview")
            note = saved(title)
            source = note["source"].replace("\\.", ".").replace("\\)", ")")
            assert source.startswith("## " + prefix + " Overview"), note
            reopen(title)
            field(prefix + " Overview")
            completed.append("native text-client space / heading / undo / redo / save / reopen: " + title)

    for structured in (False, True):
        title = "Editing native heading continuation " + str(structured)
        source = "## 1. Parent\n\nDraft"
        if structured:
            source += "\n\n| A | B |\n| --- | --- |\n| one | two |\n"
        imported(title, source)
        pointer(field("Draft") if structured else field(label="Document editor"))
        key("Ctrl+End")
        for _ in range(5):
            key("Backspace")
        type_text("###")
        native_space()
        field("1.1 " if structured else "1. Parent\n1.1 ")
        type_text("Child")
        field("1.1 Child" if structured else "1. Parent\n1.1 Child")
        source = saved(title)["source"].replace("\\.", ".")
        assert "### 1.1 Child" in source, source
        reopen(title)
        field("1.1 Child" if structured else "1. Parent\n1.1 Child")
        completed.append("native text-client space / new child heading / continuation / reopen: " + str(structured))

    for rapid in (False, True):
        title = "Editing numbered heading " + str(rapid)
        new_note(title)
        events = [dict(type="text", text=char) for char in "## 1. Overview **bold**"]
        if rapid:
            call("input", events=events)
        else:
            for event in events:
                call("input", events=[event])
        field("1. Overview bold", label="Document editor")
        source = saved(title)["source"]
        assert source.startswith("## ") and "Overview **bold**" in source, source
        reopen(title)
        field("1. Overview bold", label="Document editor")
        completed.append("numbered H2 / inline bold / save / reopen / rapid input: " + str(rapid))

    for numbered in (False, True):
        title = "Editing automatic heading numbers " + str(numbered)
        new_note(title)
        events = []
        for index, text in enumerate(("## " + ("1. " if numbered else "") + "Parent", "### Child", "### Sibling", "## Next")):
            if index:
                events.extend([dict(type="key", key="Enter"), dict(type="key", key="Enter")])
            events.extend(dict(type="text", text=char) for char in text)
        call("input", events=events)
        expected = "1. Parent\n\n1.1 Child\n\n1.2 Sibling\n\n2. Next" if numbered else "Parent\n\nChild\n\nSibling\n\nNext"
        field(expected, label="Document editor")
        source = saved(title)["source"]
        assert "### " in source, source
        reopen(title)
        # Explicit empty paragraphs round-trip through the block editor.
        # Verify every heading and gap instead of relying on the editor type.
        for heading in expected.split("\n\n"):
            field(heading)
        assert saved(title)["source"].count("<p/>") == 3
        completed.append("automatic heading numbers / child / sibling / parent / rapid input / reopen: " + str(numbered))

    for structured in (False, True):
        title = "Editing heading reordering " + str(structured)
        source = "## 1. First\n\n## 2. Second\n\n## 3. Third"
        if structured:
            source += "\n\n| A | B |\n| --- | --- |\n| one | two |\n"
        imported(title, source)

        def numbered_fields(titles):
            if structured:
                for text in titles:
                    field(text)
            else:
                field("\n".join(titles), label="Document editor")

        original = ["1. First", "2. Second", "3. Third"]
        numbered_fields(original)
        pointer(field("2. Second") if structured else field(label="Document editor"))
        if not structured:
            key("Ctrl+Home")
            key("Down")
        key("Cmd+Ctrl+3")
        demoted = ["1. First", "1.1 Second", "2. Third"]
        numbered_fields(demoted)
        key("Cmd+Z")
        numbered_fields(original)
        key("Cmd+Shift+Z")
        numbered_fields(demoted)
        source = saved(title)["source"].replace("\\.", ".")
        assert "### 1.1 Second" in source, source
        reopen(title)
        numbered_fields(demoted)

        pointer(field("1.1 Second") if structured else field(label="Document editor"))
        if not structured:
            key("Ctrl+Home")
            key("Down")
        key("Cmd+Ctrl+2")
        numbered_fields(original)

        # Removing a heading from the sequence must renumber later siblings.
        pointer(field("1. First") if structured else field(label="Document editor"))
        key("Ctrl+Home")
        key("Cmd+Ctrl+0")
        numbered_fields(["1. First", "1. Second", "2. Third"])
        source = saved(title)["source"].replace("\\.", ".")
        assert not source.startswith("## "), source
        assert "## 1. Second" in source and "## 2. Third" in source, source
        key("Cmd+Z")
        numbered_fields(original)
        completed.append("heading level / renumber / undo / redo / save / reopen: " + str(structured))

    for structured in (False, True):
        title = "Editing heading conversion history " + str(structured)
        source = "## 1. Parent\n\nDraft 世界"
        if structured:
            source += "\n\n| A | B |\n| --- | --- |\n| one | two |\n"
        imported(title, source)
        pointer(field("Draft 世界") if structured else field(label="Document editor"))
        key("Ctrl+End")
        key("Cmd+Home")
        key("Right")
        key("Right")
        key("Cmd+Ctrl+3")
        converted = "1.1 Draft 世界" if structured else "1. Parent\n1.1 Draft 世界"
        target = field(converted)
        offset = 0 if structured else len("1. Parent\n")
        assert target["selection"] == [offset + 6, offset + 6], target
        key("Cmd+Z")
        original = "Draft 世界" if structured else "1. Parent\nDraft 世界"
        field(original)
        undone = saved(title)["source"].replace("\\.", ".")
        assert "### " not in undone and "1.1 Draft" not in undone, undone
        key("Cmd+Shift+Z")
        field(converted)
        type_text("X")
        field(converted.replace("Draft", "DrXaft"))
        saved(title)
        reopen(title)
        field(converted.replace("Draft", "DrXaft"))
        completed.append("paragraph to heading / caret / single undo / redo / continued typing / reopen: " + str(structured))

    # A stationary pointer must not trap the keyboard highlight over Math.
    def option_selected(node):
        # The automation protocol exposes the raw checked bit for list options;
        # platform accessibility and ui.Tester present it as selected.
        return bool({"checked", "selected"}.intersection(node.get("states", [])))

    title = "Editing insert menu navigation"
    new_note(title)
    pointer(control("Insert block"))
    pointer(control("Insert Math"), "pointerMove")
    for name in ("Table", "Callout", "Code block", "Quote", "To-do list", "Bullet list", "Heading"):
        key("Up")
        option = control("Insert " + name)
        assert option_selected(option), (name, option)
    option_names = ("Heading", "Bullet list", "To-do list", "Quote", "Code block", "Callout", "Table", "Math", "Divider",
                    "Image or attachment…", "Columns", "Countdown", "Inline equation", "Reminder", "Button", "Document link", "Child pages")
    for name in option_names[1:]:
        key("Down")
        option = control(name if name == "Image or attachment…" else "Insert " + name)
        assert option_selected(option) and "offscreen" not in option.get("states", []), (name, option)
    key("Down")
    assert option_selected(control("Insert Heading"))
    key("Enter")
    empty = field("")
    wait("Keyboard insertion did not focus the heading", lambda tree: any(n.get("label") == empty["label"] and focused(n) for n in tree))
    type_text("Keyboard heading")
    field("Keyboard heading")
    assert "## Keyboard heading" in saved(title)["source"]
    pointer(control("Insert block"))
    type_text("no-such-block")
    control("No matching blocks")
    key("Enter")
    control("Search blocks")
    key("Cmd+A")
    type_text("list")
    key("Down")
    assert option_selected(control("Insert To-do list"))
    key("Cmd+A")
    # The query and Enter can arrive before a frame publishes the bound text.
    call("input", events=[dict(type="text", text="callout"), dict(type="key", key="Enter")])
    type_text("Filtered callout")
    field("Filtered callout")
    assert "Filtered callout" in saved(title)["source"]
    completed.append("insert menu / stationary pointer / every option / scroll / filtering / Enter / caret")

    # A slash command must consume '/', create empty content, and leave a live
    # caret. Two separate inputs expose focus loss after the first transaction.
    for kind in ("Heading", "Quote", "Callout", "Code block"):
        title = "Editing insertion " + kind
        new_note(title)
        type_text("/")
        pointer(control("Insert " + kind))
        empty = field("")
        wait("Insertion did not focus its empty field: " + kind, lambda tree: any(n.get("label") == empty["label"]
             and focused(n) for n in tree))
        source = saved(title)["source"]
        assert "A thought worth keeping" not in source and "Write something" not in source, source
        type_text("Authored 世界")
        field("Authored 世界")
        type_text(" 🌱")
        field("Authored 世界 🌱")
        source = saved(title)["source"]
        reopen(title)
        # Markdown requires a newline before the closing code fence.
        field("Authored 世界 🌱" + ("\n" if kind == "Code block" else ""))
        assert saved(title)["source"] == source, "Reopening changed authored content"
        completed.append("slash insertion / empty content / focus / reopen: " + kind)

    # Ordinary deletion belongs to the focused empty editor. Do not select a
    # drag handle or click again after deletion to repair the expected caret.
    for kind, action_label in (("Quote", "Block actions blockquote"),
                               ("Code block", "Code language"),
                               ("Callout", "Callout type"),
                               ("Bullet list", "Block actions ul"),
                               ("To-do list", "Block actions checkbox")):
        for deletion in ("Backspace", "Delete"):
            title = "Editing empty block direct deletion " + kind + " " + deletion
            new_note(title)
            type_text("/")
            pointer(control("Insert " + kind))
            focused_field("")
            original = saved(title)["source"]
            if kind in ("Code block", "Callout", "To-do list"):
                reopen(title)
                pointer(field(""))
            key(deletion)
            focused_field("")
            assert not any(n.get("label") == action_label for n in nodes()), (kind, deletion)
            clean = saved(title)["source"]
            assert clean != original, (kind, deletion, original)
            key("Cmd+Z")
            control(action_label)
            assert saved(title)["source"] == original, "One undo did not restore the empty block"
            key("Cmd+Shift+Z")
            focused_field("")
            assert saved(title)["source"] == clean, "Redo changed empty-block deletion"
            type_text("Continue")
            type_text(" writing 世界")
            focused_field("Continue writing 世界")
            source = saved(title)["source"]
            reopen(title)
            field("Continue writing 世界")
            assert saved(title)["source"] == source, "Reopening changed continued text"
            completed.append("empty block / direct " + deletion + " / single undo and redo / typing / reopen: " + kind)

    # Reopened quotes and code after a list reproduce the reported boundary.
    for block, action_label in (("<blockquote><p/></blockquote>", "Block actions blockquote"),
                                ("<pre lang=\"go\"><code/></pre>", "Code language"),
                                ("<callout>\n<p/>\n</callout>", "Callout type")):
        for deletion in ("Backspace", "Delete"):
            title = "Editing reopened empty container " + action_label + " " + deletion
            imported(title, "<ul><li>Water Cube</li><li>National Stadium</li></ul>\n\n" + block + "\n\n<p>Following</p>")
            original = saved(title)["source"]
            reopen(title)
            pointer(field(""))
            key(deletion)
            focused_field("")
            assert not any(n.get("label") == action_label for n in nodes())
            field("Water Cube")
            field("National Stadium")
            field("Following")
            key("Cmd+Z")
            assert saved(title)["source"] == original
            key("Cmd+Shift+Z")
            focused_field("")
            type_text("Body after list")
            focused_field("Body after list")
            source = saved(title)["source"]
            reopen(title)
            field("Body after list")
            field("National Stadium")
            assert saved(title)["source"] == source
            completed.append("reopened empty container after list / " + deletion + " / neighbors / undo / continued typing: " + action_label)

    # Ordinary lists and headings reopen in the rich editor. Exercise production
    # editor selection without forcing a structured representation.
    for source in (">\n", "-\n", "1.\n", "##\n"):
        for deletion in ("Backspace", "Delete"):
            title = "Editing empty Markdown " + source.strip() + " " + deletion
            imported(title, source)
            original = saved(title)["source"]
            reopen(title)
            pointer(field(""))
            key(deletion)
            focused_field("")
            clean = saved(title)["source"]
            assert clean != original, (source, deletion)
            key("Cmd+Z")
            assert saved(title)["source"].strip() == original.strip()
            key("Cmd+Shift+Z")
            type_text("Ordinary body")
            focused_field("Ordinary body")
            content = saved(title)["source"]
            assert content.strip() == "Ordinary body", content
            reopen(title)
            field("Ordinary body")
            completed.append("empty Markdown / " + deletion + " / undo / redo / body typing / reopen: " + source.strip())

    # Deleting a selection empties the block first. Only a later key exits it.
    for kind, action_label in (("Quote", "Block actions blockquote"),
                               ("Code block", "Code language"),
                               ("Callout", "Callout type")):
        for deletion in ("Backspace", "Delete"):
            title = "Editing text then block deletion " + kind + " " + deletion
            new_note(title)
            type_text("/")
            pointer(control("Insert " + kind))
            type_text("Authored 世界")
            focused_field("Authored 世界")
            key("Cmd+A")
            key(deletion)
            focused_field("")
            control(action_label)
            key(deletion)
            focused_field("")
            assert not any(n.get("label") == action_label for n in nodes())
            type_text("Outside")
            focused_field("Outside")
            source = saved(title)["source"]
            reopen(title)
            field("Outside")
            assert saved(title)["source"] == source
            completed.append("selected text deletion preserves block / second " + deletion + " exits / reopen: " + kind)

    # Read mode must never treat a boundary key as an editing command.
    for block in ("<blockquote><p/></blockquote>", "<pre><code/></pre>", "<callout>\n<p/>\n</callout>"):
        title = "Editing protected empty block " + block.split(">")[0]
        imported(title, block)
        source = saved(title)["source"]
        pointer(control("Read"))
        pointer(field(""))
        key("Backspace")
        key("Delete")
        assert saved(title)["source"] == source, "Read-only empty block was deleted"
        completed.append("read-only empty container / Backspace and Delete preserve source: " + block)

    # Native edit commands and both keyboard bindings share progressive scope.
    for shortcut in ("Ctrl+A", "Cmd+A", "menu"):
        title = "Editing progressive select all " + shortcut
        imported(title, "> [!NOTE]\n> First\n>\n> Current\n>\n> Last\n\nOutside\n")
        pointer(field("Current"))

        def select_all():
            if shortcut == "menu":
                call("menu", path=["Edit", "Select All"])
            else:
                key(shortcut)

        select_all()
        control("Press Ctrl+A or ⌘A again to select the entire document body")
        key("Backspace")
        field("")
        field("First")
        field("Outside")
        key("Cmd+Z")
        pointer(field("Current"))
        select_all()
        select_all()
        # Autosave can replace the transient status message. Verify the actual
        # selection, then deletion and undo, rather than a timing-sensitive toast.
        wait("Select All did not include both body blocks", lambda tree: len([
            n for n in tree if n.get("label", "").startswith("Block actions ")
            and "checked" in n.get("states", [])]) == 2)
        control("Delete selected blocks")
        key("Backspace")
        focused_field("")
        source = saved(title)["source"]
        assert "Current" not in source and "Outside" not in source, source
        focused_field("")
        key("Cmd+Z")
        field("Current")
        field("Outside")
        completed.append("progressive select all / hint / local deletion / body deletion / undo: " + shortcut)

    for arrow in ("Down", "Right"):
        title = "Editing callout exit " + arrow
        imported(title, "> [!NOTE]\n> Inside 世界")
        pointer(field("Inside 世界"))
        key("Cmd+End")
        key(arrow)
        assert focused(field("")), "Callout boundary did not create a body caret"
        type_text("Outside")
        type_text(" body")
        field("Outside body")
        field("Inside 世界")
        source = saved(title)["source"]
        assert "\n\nOutside body" in source, source
        key("Cmd+Z")
        key("Cmd+Z")
        field("Inside 世界")
        pointer(control("Write below callout"))
        type_text("Mouse body")
        field("Mouse body")
        pointer(control("Callout type"))
        act("danger")
        source = saved(title)["source"]
        assert ":::danger" in source and "Mouse body" in source, source
        reopen(title)
        field("Inside 世界")
        field("Mouse body")
        assert saved(title)["source"] == source, "Reopening lost callout type or body"
        act("Read")
        labels = {n.get("label") for n in nodes()}
        assert "Callout type" not in labels and "Write below callout" not in labels, labels
        act("Write")
        completed.append("callout arrow / mouse exit / type / undo / reopen / read-only: " + arrow)

    for deletion in ("Backspace", "Delete", "toolbar"):
        title = "Editing deletion " + deletion
        new_note(title)
        type_text("/")
        pointer(control("Insert Callout"))
        pointer(control("Block actions callout"))
        key("Cmd+A")
        if deletion == "toolbar":
            # A single selected block also needs an explicit deletion affordance.
            pointer(control("Delete selected blocks"))
        else:
            key(deletion)
        empty = field("")
        pointer(empty)
        type_text("Continue")
        field("Continue")
        type_text(" after deletion")
        field("Continue after deletion")
        key("Cmd+Z")
        field("")
        key("Cmd+Z")
        control("Block actions callout")
        key("Cmd+Shift+Z")
        empty = field("")
        pointer(empty)
        type_text("Recovered body")
        field("Recovered body")
        source = saved(title)["source"]
        assert "Recovered body" in source and "[!NOTE]" not in source, source
        reopen(title)
        field("Recovered body")
        completed.append("delete last block / click / type / undo / redo / reopen: " + deletion)

    for following in ("", "\n\n## Following heading\n"):
        title = "Editing code boundary " + ("heading" if following else "end")
        imported(title, "```go\npackage main\n```" + following)
        code = field(label="Code editor")
        original = code.get("value", "")
        pointer(code)
        key("Cmd+End")
        key("Enter")
        type_text("// inside")
        field(original + "\n// inside", "Code editor")
        # Native Edit undo and keyboard redo must share history and caret.
        call("menu", path=["Edit", "Undo"])
        field(label="Code editor")
        key("Cmd+Shift+Z")
        field(original + "\n// inside", "Code editor")
        key("Cmd+End")
        key("Shift+Left")
        key("Down")
        assert focused(field(label="Code editor")), "Selection escaped code"
        key("Cmd+End")
        key("Down")
        if following:
            focused_field("Following heading")
        else:
            focused_field("")
            key("Backspace")
            focused_field(label="Code editor")
            key("Down")
            focused_field("")
        pointer(control("Write below code"))
        type_text("Outside")
        field("Outside")
        type_text(" code 世界")
        field("Outside code 世界")
        if following:
            field("Following heading")
        source = saved(title)["source"]
        parts = source.split("```")
        assert len(parts) == 3 and "// inside" in parts[1] and "Outside code 世界" in parts[2], source
        if following:
            assert parts[2].index("Outside code 世界") < parts[2].index("## Following heading"), source
        reopen(title)
        field("Outside code 世界")
        field(original + "\n// inside\n", "Code editor")
        act("Read")
        assert not any(n.get("label") == "Write below code" for n in nodes()), "Read mode exposes insertion"
        act("Write")
        completed.append("code Enter / selection / undo / keyboard exit / pointer exit / reopen / read-only: " + title)

    for forward in (False, True):
        title = "Editing paragraph join " + str(forward)
        imported(title, "First\n\nSecond\n\n```go\nkeep\n```\n")
        target = field("First" if forward else "Second")
        pointer(target)
        key("Cmd+End" if forward else "Cmd+Home")
        key("Shift+Right" if forward else "Shift+Left")
        assert focused(target), "Selection extension crossed the paragraph boundary"
        key("Delete" if forward else "Backspace")
        type_text(" joined ")
        field("First joined Second")
        key("Cmd+Z")
        field("FirstSecond")
        key("Cmd+Z")
        field("First")
        field("Second")
        key("Cmd+Shift+Z")
        field("FirstSecond")
        source = saved(title)["source"]
        reopen(title)
        field("FirstSecond")
        assert saved(title)["source"] == source
        completed.append("paragraph boundary join / Shift selection / undo / redo / reopen: " + str(forward))

    title = "Editing text selection"
    imported(title, "> [!NOTE]\n> Inside\n\nOutside\n")
    pointer(field("Inside"))
    key("Cmd+A")
    key("Backspace")
    type_text("Replacement")
    field("Replacement")
    field("Outside")
    source = saved(title)["source"]
    assert "[!NOTE]" in source and "Replacement" in source and "Outside" in source, source
    completed.append("text selection deletion preserves callout and adjacent paragraph")

    title = "Editing heading geometry"
    imported(title, "# 万卡集群从开始到运行起来模型需要什么？\n\nBody stays here.\n\n```go\nx\n```\n")
    heading = field("万卡集群从开始到运行起来模型需要什么？")
    # The native CI desktop can wrap CJK headings onto two lines. Verify
    # adjacency and reflow instead of assuming a fixed single-line height.
    body = field("Body stays here.")
    gap = body["bounds"]["y"] - heading["bounds"]["y"] - heading["bounds"]["height"]
    assert -1 <= gap <= 12, (heading, body)
    original_height = heading["bounds"]["height"]
    pointer(heading)
    key("Cmd+A")
    type_text("Short")
    short = field("Short")
    assert 0 < short["bounds"]["height"] <= original_height, (heading, short)
    key("Cmd+Z")
    heading = field("万卡集群从开始到运行起来模型需要什么？")
    pointer(heading, "pointerMove")
    handle, fold = control("Block actions h1"), control("Collapse section 万卡集群从开始到运行起来模型需要什么？")
    h, f = handle["bounds"], fold["bounds"]
    assert f["x"] + f["width"] <= h["x"], "Fold control covers the heading action handle"
    source = saved(title)["source"]
    pointer(fold)
    assert not any(n.get("value") == "Body stays here." for n in nodes()), "Fold left its body visible"
    pointer(control("Expand section 万卡集群从开始到运行起来模型需要什么？"))
    field("Body stays here.")
    assert saved(title)["source"] == source, "Folding rewrote source"
    pointer(control("Block actions h1"))
    key("Delete")
    type_text("Continued ")
    wait("Deleting heading lost the next paragraph or typing focus", lambda tree: any("Continued " in n.get("value", "")
         and "Body stays here." in n.get("value", "") for n in tree))
    assert "# 万卡" not in saved(title)["source"]
    completed.append("heading geometry / separate controls / fold / delete / continued typing")

    # Both trash layouts must expose only trash actions; restoring must return
    # to the same saved content, and purging must remove the persisted record.
    for layout in ("Grid view", "List view"):
        title = "Editing trash " + layout
        new_note(title)
        type_text("Retained body 世界")
        field("Retained body 世界")
        note = saved(title)
        act("Document actions")
        act("Move to Trash")
        act("Trash")
        act(layout)
        pointer(control("Actions " + title))
        labels = {n.get("label") for n in nodes()}
        assert not labels.intersection({"Note name", "Rename", "Move to Trash", "Create copy", "Move…"}), labels
        act("Restore note")
        act("All notes")
        act("Note " + title)
        field("Retained body 世界")
        assert saved(title)["source"] == note["source"]
        act("Document actions")
        act("Move to Trash")
        act("Trash")
        pointer(control("Note " + title))
        act("Delete permanently")
        assert not (root / "library" / "trash" / (note["id"] + ".md")).exists()
        assert not (root / "library" / "notes" / (note["id"] + ".md")).exists()
        completed.append("trash actions / restore content / permanent removal: " + layout)

    # Enter in the document title must hand typing to the body without a click.
    for source in ("", "Existing body", "```text\nExisting body\n```\n"):
        for rapid in (False, True):
            title = "Editing title Enter " + str(len(source)) + " " + str(rapid)
            if source:
                imported(title, source)
            else:
                new_note(title)
            pointer(control("Document title"))
            key("Cmd+End")
            events = [dict(type="key", key="Enter"), dict(type="text", text="Body probe "),
                      dict(type="text", text="continued ")]
            if rapid:
                call("input", events=events)
            else:
                for event in events:
                    call("input", events=[event])
            expected = "Body probe continued " + ("Existing body" if source else "")
            wait("Title Enter did not hand typing to the body", lambda tree: any(
                n.get("label") != "Document title" and n.get("value", "").rstrip("\n") == expected
                and focused(n) for n in tree))
            note = saved(title)
            assert note["title"] == title and expected in note["source"], note
            reopen(title)
            wait("Title Enter body did not persist", lambda tree: any(
                n.get("label") != "Document title" and expected.strip() in n.get("value", "") for n in tree))
            completed.append("title Enter / body focus / typing / save / reopen / rapid: " + str(rapid))

    print("Native editing regressions passed:\n" + "\n".join("  - " + case for case in completed), flush=True)
