"""Table regressions through real native menus, pointer events and keyboard input."""
import time
from library_records import note_paths, read_note


def exercise_tables(call, nodes, act, key, root, fixtures):
    def wait(description, predicate):
        deadline = time.monotonic() + 5
        while True:
            tree = nodes()
            result = predicate(tree)
            if result:
                return result
            if time.monotonic() >= deadline:
                raise AssertionError(f"{description}: {[(n.get('label'), n.get('value')) for n in tree]}")
            time.sleep(.04)

    def control(label):
        return wait(label, lambda tree: next((n for n in tree if n.get("label") == label), None))

    def field(value):
        return wait(value, lambda tree: next((n for n in tree if n.get("label", "").startswith("Block text ")
                                            and n.get("value") == value), None))

    def pointer(node, **extra):
        b = node["bounds"]
        call("input", events=[dict(type="click", x=b["x"]+b["width"]/2,
                                   y=b["y"]+min(12, b["height"]/2), **extra)])

    def text(value):
        call("input", events=[dict(type="text", text=value)])

    def save(title):
        call("menu", path=["File", "Save"])
        return next(read_note(p)["source"] for p in note_paths(root / "library" / "notes")
                    if read_note(p)["title"] == title)

    def new(title):
        call("menu", path=["File", "New note"])
        act("Document title", "setValue", text=title)
        pointer(control("Document editor"))

    def imported(title, source):
        path = fixtures / "Fixture import.md"
        old = path.read_bytes()
        try:
            path.write_text(source)
            call("menu", path=["File", "Import…"])
            wait("Import did not open", lambda tree: any(n.get("label") == "Document title"
                 and n.get("value") == "Fixture import" for n in tree))
            act("Document title", "setValue", text=title)
        finally:
            path.write_bytes(old)

    def reopen(title):
        pointer(control("Back to library"))
        act("All notes")
        act("Note " + title)

    title = "Table native keyboard"
    new(title)
    text("/")
    text("table")
    key("Enter")
    control("Create table")
    key("Escape")
    assert save(title).strip() == "/", "Cancel consumed slash or inserted a table"
    pointer(control("Document editor"))
    key("Cmd+A")
    text("/")
    text("table")
    key("Enter")
    pointer(control("Table size 2 3"))
    for value in ["Name", "Status", "Notes", "Alpha", "Ready", "Last"]:
        text(value)
        key("Tab")
    text("New row")
    field("New row")
    key("Shift+Tab")
    text("!")
    field("Last!")
    key("Enter")
    text("Second line")
    field("Second line")
    key("Cmd+Enter")
    text("After table")
    field("After table")
    source = save(title)
    assert "<p>/</p>" not in source and not source.startswith("/"), source
    assert all(word in source for word in ["Name", "Status", "Last!", "Second line", "New row", "After table"]), source

    # Native text selection changes only selected words in a cell.
    pointer(field("Alpha"))
    key("Home")
    key("Shift+Right")
    key("Cmd+U")
    source = save(title)
    assert "<u>A</u>lpha" in source, source
    key("Cmd+Z")
    pointer(control("Select row 2"))
    key("Cmd+B")
    pointer(control("Cell background"))
    pointer(control("light-green"))
    source = save(title)
    assert "<b>Alpha</b>" in source and "<b>Ready</b>" in source and "<b>Name</b>" not in source, source
    assert source.count('background-color="light-green"') == 3, source

    # The menu bar must use the selected range, not the stale cell caret.
    call("menu", path=["Edit", "Copy"])
    pointer(control("Select row 3"))
    call("menu", path=["Edit", "Paste"])
    source = save(title)
    assert source.count("Alpha") == 2 and source.count("Second line") == 2, source
    key("Cmd+Z")
    undone = save(title)
    assert undone.count("Alpha") == 1, "Grid paste was not one undo step: " + undone
    key("Cmd+Shift+Z")
    assert save(title).count("Alpha") == 2
    key("Cmd+Z")
    pointer(control("Select column B"))
    pointer(control("Align"))
    pointer(control("Center"))
    pointer(control("Table actions"))
    pointer(control("Delete column"))
    source = save(title)
    assert "Status" not in source and "Ready" not in source and "Alpha" in source, source
    key("Cmd+Z")
    assert "Status" in save(title)
    pointer(control("Select row 3"))
    key("Backspace")
    assert "New row" not in save(title) and "After table" in save(title)
    key("Cmd+Z")
    before = save(title)
    reopen(title)
    assert save(title) == before, "Reopening rewrote table content or formatting"
    for mode in ["Source", "Read", "Split", "Write"]:
        pointer(control(mode))
    assert save(title) == before, "Mode changes rewrote table content"

    # Insertion into an existing document must preserve surrounding blocks and
    # page geometry even when the table needs its own horizontal scrollbar.
    title = "Table native middle insertion"
    imported(title, '<p>Before paragraph</p><p>Insert here</p><p>After paragraph</p>')
    bounds = control("Document title")["bounds"]
    pointer(field("Insert here"))
    key("Cmd+A")
    text("/")
    text("table")
    key("Enter")
    pointer(control("Table size 3 9"))
    text("First cell")
    field("First cell")
    after = control("Document title")["bounds"]
    assert abs(bounds["x"]-after["x"]) <= 1 and abs(bounds["width"]-after["width"]) <= 1, (bounds, after)
    source = save(title)
    assert source.count("<table") == 1 and "<p>/</p>" not in source and "Insert here" not in source, source
    assert source.index("Before paragraph") < source.index("<table") < source.index("After paragraph"), source
    reopen(title)
    assert save(title) == source

    # Editing an imported Markdown table cannot leave a dangling delimiter.
    title = "Table native Markdown delimiter"
    imported(title, "Before\n\n| A | B |\n| --- | --- |\n| One | Two |\n\nAfter")
    pointer(field("Two"))
    key("Cmd+A")
    text("Changed")
    source = save(title)
    assert "Changed" in source and "Before" in source and "After" in source, source
    assert not any(line.strip() == "|" for line in source.splitlines()), source
    reopen(title)
    assert save(title) == source
    print("Native tables passed: slash/cancel, middle insertion, layout, immediate typing, Tab/Shift+Tab, multiline, exit, text/range formatting, native copy/paste, column deletion, clear, undo/redo, modes and reopen.", flush=True)
