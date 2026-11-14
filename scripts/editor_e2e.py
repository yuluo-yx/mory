"""Editing regressions driven through native pointer and keyboard events.

Accessibility locates targets and reads results; it must not repair focus after
an insertion, deletion, undo or boundary transition under test.
"""
import json
import time


def exercise_editor(call, nodes, act, key, root, fixtures):
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

    def saved(title):
        call("menu", path=["File", "Save"])
        nodes()
        return next(json.loads(p.read_text()) for p in (root / "library" / "notes").glob("*.json")
                    if json.loads(p.read_text())["title"] == title)

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
        assert "A thought worth keeping" not in source and "Write something" not in source and "/" not in source, source
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
            next_field = field("Following heading")
            assert focused(next_field), "Boundary did not enter following heading"
        else:
            assert focused(field("")), "Boundary did not create a writable paragraph"
            key("Backspace")
            assert focused(field(label="Code editor")), "Empty paragraph could not return to code"
            key("Down")
            assert focused(field("")), "Repeated exit lost the writable paragraph"
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
    assert heading["bounds"]["height"] < 100, heading
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
        assert not (root / "library" / "trash" / (note["id"] + ".json")).exists()
        assert not (root / "library" / "notes" / (note["id"] + ".json")).exists()
        completed.append("trash actions / restore content / permanent removal: " + layout)

    print("Native editing regressions passed:\n" + "\n".join("  - " + case for case in completed), flush=True)
