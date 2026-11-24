"""Everyday writing sequences using the native editor's actual input path."""
import time

from library_records import note_paths, read_note


def exercise_writing(call, nodes, act, key, root, fixtures):
    completed = []

    def check(name):
        completed.append(name)
        print("Writing E2E passed: " + name, flush=True)

    def wait(description, predicate):
        deadline = time.monotonic() + 5
        while True:
            tree = nodes()
            result = predicate(tree)
            if result:
                return result
            if time.monotonic() >= deadline:
                snapshot = [(node.get("label"), node.get("value"), node.get("selection")) for node in tree]
                raise AssertionError(f"{description}; tree={snapshot}")
            time.sleep(0.03)

    def control(label):
        return wait("Missing control " + label, lambda tree: next((node for node in tree
                    if node.get("label") == label and "offscreen" not in node.get("states", [])), None))

    def field(value):
        return wait("Missing body text " + repr(value), lambda tree: next((node for node in tree
                    if node.get("label", "").startswith("Block text ") or node.get("label") == "Document editor"
                    if node.get("value", "") == value), None))

    def pointer(node):
        box = node["bounds"]
        call("input", events=[dict(type="click", x=box["x"]+box["width"]/2,
                                   y=box["y"]+min(12, box["height"]/2))])

    def type_text(text):
        call("input", events=[dict(type="text", text=text)])

    def replace(label, text):
        pointer(control(label))
        call("input", events=[dict(type="key", key="Cmd+A"), dict(type="text", text=text)])
        assert control(label).get("value") == text

    def saved(title):
        call("menu", path=["File", "Save"])
        nodes()
        matches = [read_note(path) for path in note_paths(root / "library" / "notes")
                   if read_note(path)["title"] == title]
        assert len(matches) == 1, (title, matches)
        return matches[0]

    def imported(title, source):
        path = fixtures / "Fixture import.md"
        previous = path.read_bytes()
        try:
            path.write_text(source, encoding="utf-8")
            call("menu", path=["File", "Import…"])
            wait("Import did not open", lambda tree: any(node.get("label") == "Document title"
                 and node.get("value") == "Fixture import" for node in tree))
            replace("Document title", title)
        finally:
            path.write_bytes(previous)

    def reopen(title):
        act("All notes")
        act("Note " + title)
        assert control("Document title").get("value") == title

    # Keyboard selections use grapheme movement; committed Unicode input must
    # survive replacement, undo, redo and subsequent typing at the same caret.
    for structured in (False, True):
        title = "Writing selection replacement " + str(structured)
        original = "前言 English 🌱 旧内容"
        source = "<p>" + original + "</p>\n\n<p>Keep this paragraph</p>" if structured else original
        imported(title, source)
        pointer(field(original))
        key("Ctrl+End")
        for _ in range(3):
            key("Shift+Left")
        type_text("新内容")
        changed = "前言 English 🌱 新内容"
        field(changed)
        key("Cmd+Z")
        field(original)
        key("Cmd+Shift+Z")
        field(changed)
        type_text("，继续写作")
        expected = changed + "，继续写作"
        target = field(expected)
        assert call("tree").get("focus") == target["id"], "Replacement lost the caret"
        note = saved(title)
        assert expected in note["source"] and "旧内容" not in note["source"]
        if structured:
            assert "Keep this paragraph" in note["source"]
        reopen(title)
        field(expected)
        assert saved(title)["source"] == note["source"]
        check("Unicode selection replacement / undo / redo / continued typing / reopen: " + str(structured))

    for structured in (False, True):
        title = "Writing emoji deletion " + str(structured)
        original = "Before 👩‍💻"
        imported(title, "<p>" + original + "</p>" if structured else original)
        pointer(field(original))
        key("Ctrl+End")
        key("Backspace")
        field("Before ")
        key("Cmd+Z")
        field(original)
        key("Cmd+Shift+Z")
        type_text("🌱 after")
        field("Before 🌱 after")
        note = saved(title)
        assert "👩" not in note["source"] and "\u200d" not in note["source"]
        reopen(title)
        field("Before 🌱 after")
        check("composed emoji deletion / whole grapheme undo / continuation: " + str(structured))

    # Turning formatting off must apply only to new typing, including after
    # reopening the document in another rendering mode.
    title = "Writing inline formatting boundaries"
    imported(title, "Intro")
    pointer(field("Intro"))
    key("Ctrl+End")
    type_text(" ")
    key("Cmd+B")
    type_text("强调")
    key("Cmd+B")
    type_text(" normal")
    note = saved(title)
    assert "**强调** normal" in note["source"], note["source"]
    for mode in ("Read", "Source", "Split", "Write"):
        pointer(control(mode))
        assert saved(title)["source"] == note["source"], mode
    field("Intro 强调 normal")
    check("bold toggle boundaries / Chinese typing / four-mode lossless round trip")

    # A complete replace-all is a single user action, even when it spans
    # multiple blocks. Undo and redo should restore the entire operation.
    for structured in (False, True):
        title = "Writing replace all " + str(structured)
        source = "<p>猫 cat 猫</p>\n\n<p>猫 stays here</p>" if structured else "猫 cat 猫\n\n猫 stays here"
        imported(title, source)
        original = saved(title)["source"]
        key("Cmd+Alt+F")
        replace("Find text", "猫")
        replace("Replace text", "小猫🌱")
        pointer(control("Replace all"))
        changed = saved(title)["source"]
        if structured:
            field("小猫🌱 cat 小猫🌱")
            field("小猫🌱 stays here")
            assert changed.count("小猫🌱") == 3, changed
        else:
            assert changed == original.replace("猫", "小猫🌱"), (original, changed)
        pointer(control("Close find"))
        tree = call("tree")
        focused = next((node for node in tree["nodes"] if node["id"] == tree.get("focus")), None)
        assert focused and (focused.get("label") == "Document editor" or focused.get("label", "").startswith("Block text ")), ("Closing find did not return focus to the body", focused)
        call("menu", path=["Edit", "Undo"])
        undone = saved(title)["source"]
        assert undone == original, ("Undo did not revert the entire replace-all", original, undone)
        call("menu", path=["Edit", "Redo"])
        assert saved(title)["source"] == changed, "Redo did not restore the entire replace-all"
        reopen(title)
        assert saved(title)["source"] == changed
        check("multi-paragraph replace-all / replacement includes query / atomic undo and redo: " + str(structured))

    title = "Writing settings interruption"
    imported(title, "Draft in progress")
    pointer(field("Draft in progress"))
    key("Ctrl+End")
    target = control("Settings")["bounds"]
    call("input", events=[dict(type="text", text=" with unsaved words"),
                           dict(type="click", x=target["x"]+target["width"]/2, y=target["y"]+target["height"]/2)])
    pointer(control("Editor"))
    pointer(control("Back from settings"))
    field("Draft in progress with unsaved words")
    pointer(field("Draft in progress with unsaved words"))
    key("Ctrl+End")
    type_text(" and more")
    expected = "Draft in progress with unsaved words and more"
    assert saved(title)["source"] == expected
    reopen(title)
    field(expected)
    check("unsaved typing / settings interruption / continued writing / reopen")

    title = "Writing list exit"
    imported(title, "<p>Before</p>\n\n- First item\n\n<p>After</p>")
    pointer(field("First item"))
    key("Ctrl+End")
    key("Enter")
    type_text("Second item")
    field("Second item")
    key("Enter")
    key("Enter")
    type_text("Back to prose")
    field("Back to prose")
    source = saved(title)["source"]
    assert "Second item" in source and "Back to prose" in source
    assert "- Back to prose" not in source and "<li>Back to prose" not in source, source
    assert "Before" in source and "After" in source
    reopen(title)
    field("Back to prose")
    check("continue list / double Enter exits empty item / surrounding paragraphs preserved")

    title = "Writing source selection"
    source = "## Heading\n\nText before\n\n```go\nfmt.Println(\"hello\")\n```\n\nTail"
    imported(title, source)
    pointer(control("Source"))
    pointer(control("Source editor"))
    key("Cmd+End")
    for _ in range(4):
        key("Shift+Left")
    type_text("末尾 🌱")
    expected = source[:-4] + "末尾 🌱"
    actual = saved(title)["source"]
    assert actual == expected, ("Source replacement", expected, actual, control("Source editor").get("selection"))
    key("Cmd+Z")
    assert saved(title)["source"] == source
    key("Cmd+Shift+Z")
    for mode in ("Split", "Read", "Write", "Source"):
        pointer(control(mode))
        assert saved(title)["source"] == expected, mode
    reopen(title)
    assert saved(title)["source"] == expected
    check("source tail selection / Unicode replacement / undo / redo / code fences survive mode changes")

    act("All notes")
    return completed
