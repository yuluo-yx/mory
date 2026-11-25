"""User journeys through the real library, sidebar, settings and saved records.

Locate controls through accessibility, then use pointer and keyboard input for
the workflow under test. Never save or repair focus before checking a transition.
"""
import json
import time

from library_records import note_paths, read_note


def exercise_workspace(call, nodes, act, key, root):
    completed = []
    library = root / "library"

    def check(name):
        completed.append(name)
        print("Workspace E2E passed: " + name, flush=True)

    def wait(description, predicate):
        deadline = time.monotonic() + 5
        while True:
            tree = nodes()
            result = predicate(tree)
            if result:
                return result
            if time.monotonic() >= deadline:
                snapshot = [(node.get("label"), node.get("value"), node.get("states")) for node in tree]
                raise AssertionError(f"{description}; tree={snapshot}")
            time.sleep(0.03)

    def control(label, role=None):
        return wait("Missing control " + label, lambda tree: next((node for node in tree
                    if node.get("label") == label and (role is None or node.get("role") == role)
                    and "offscreen" not in node.get("states", [])), None))

    def event(node, kind="click", offset=None, **extra):
        box = node["bounds"]
        assert box["width"] > 0 and box["height"] > 0, node
        return dict(type=kind, x=box["x"] + (offset if offset is not None else box["width"] / 2),
                    y=box["y"] + box["height"] / 2, **extra)

    def pointer(label, role=None, offset=None, **extra):
        call("input", events=[event(control(label, role), offset=offset, **extra)])

    def replace(label, text):
        pointer(label, "textField")
        call("input", events=[dict(type="key", key="Cmd+A"), dict(type="key", key="Backspace"),
                               *([dict(type="text", text=text)] if text else [])])
        assert control(label, "textField").get("value", "") == text

    def opened(title):
        return wait("Document did not open: " + title, lambda tree: next((node for node in tree
                    if node.get("label") == "Document title" and node.get("value") == title), None))

    def records(directory="notes"):
        return [read_note(path) for path in note_paths(library / directory)]

    def stored(title, directory="notes"):
        matches = [note for note in records(directory) if note["title"] == title]
        assert len(matches) == 1, (title, directory, matches)
        return matches[0]

    def settings():
        return json.loads((library / "settings.json").read_text())

    def new_note(title, body):
        call("menu", path=["File", "New note"])
        replace("Document title", title)
        key("Enter")
        call("input", events=[dict(type="text", text=body)])
        wait("Title Enter did not focus the body", lambda tree: any(node.get("value") == body
             and node.get("label") != "Document title" for node in tree))

    def sidebar_menu(title):
        row = control("Open " + title)
        call("input", events=[event(row, "pointerMove", offset=50)])
        row = control("Open " + title)
        call("input", events=[event(row, offset=row["bounds"]["width"] - 15)])

    def home():
        pointer("All notes", offset=50)
        control("Search notes")

    def snapshot():
        return {note["id"]: (note["title"], note["source"], note.get("folder", ""), note.get("parent", ""))
                for note in records()}

    folder = "Journey workspace"
    pointer("New folder", "button")
    replace("Folder title", folder)
    pointer("Save folder")
    pointer("Folder " + folder, offset=50)
    first, second = "Journey Alpha", "Journey Beta"
    new_note(first, "First body")
    new_note(second, "Second body")
    pointer("Open " + first, offset=50)
    opened(first)
    pointer("Document editor")
    key("Cmd+End")
    target = control("Open " + second)
    call("input", events=[dict(type="text", text=" preserved on switch"), event(target, offset=50)])
    opened(second)
    assert stored(first)["source"] == "First body preserved on switch"
    pointer("Open " + first, offset=50)
    opened(first)
    assert control("Document editor").get("value") == "First body preserved on switch"
    check("title-to-body typing / immediate document switch / save / reopen")

    # Settings must yield to a navigation click and keep the pending document.
    pointer("Settings")
    pointer("Keyboard shortcuts")
    pointer("Folder " + folder, offset=50)
    control("Search notes")
    assert not any(node.get("label") == "Back from settings" for node in nodes())
    assert stored(first)["source"] == "First body preserved on switch"
    assert any(node.get("label") == "Note " + first for node in nodes())
    check("settings to folder navigation / document preservation")

    # A global search is a different scope from clicking a specific folder.
    home()
    replace("Search notes", "no-such-journey-document")
    assert not any(node.get("label", "").startswith("Note Journey") for node in nodes())
    pointer("Folder " + folder, offset=50)
    assert control("Search notes").get("value", "") == "", "Folder inherited an unrelated search"
    control("Note " + first)
    control("Note " + second)
    replace("Search notes", "preserved on switch")
    control("Note " + first)
    assert not any(node.get("label") == "Note " + second for node in nodes())
    replace("Search notes", "")
    check("empty search / folder scope reset / full-text search / clear")

    # Renaming the folder currently being viewed must update its active filter.
    pointer("Folder " + folder, offset=50, button="right")
    pointer("Edit folder…")
    renamed = folder + " renamed"
    replace("Folder title", renamed)
    pointer("Save folder")
    control("Folder " + renamed)
    control("Note " + first)
    assert stored(first).get("folder") == renamed
    assert stored(second).get("folder") == renamed
    assert folder not in settings()["folders"]
    check("active folder rename / filter follows name / records persisted")

    # The sidebar border is a captured drag target; save only on release.
    divider = control("Resize sidebar", "splitter")
    box = divider["bounds"]
    x, y = box["x"] + box["width"] / 2, box["y"] + box["height"] / 2
    before = snapshot()
    call("input", events=[dict(type="pointerDown", x=x, y=y),
                           dict(type="pointerMove", x=x + 110, y=y)])
    assert settings().get("sidebarWidth", 0) == 0, "Drag wrote preferences before release"
    call("input", events=[dict(type="pointerUp", x=x + 110, y=y)])
    wait("Sidebar width was not saved", lambda tree: settings().get("sidebarWidth") == 350)
    widened = control("Folder " + renamed)["bounds"]
    assert widened["width"] == 325, widened
    assert snapshot() == before, "Resizing changed document records"
    # Restore the default via the same handle, keeping later geometry stable.
    divider = control("Resize sidebar", "splitter")
    box = divider["bounds"]
    x = box["x"] + box["width"] / 2
    call("input", events=[dict(type="pointerDown", x=x, y=y),
                           dict(type="pointerMove", x=x - 110, y=y),
                           dict(type="pointerUp", x=x - 110, y=y)])
    wait("Sidebar default width was not saved", lambda tree: settings().get("sidebarWidth") == 240)
    check("sidebar border drag / live width / persisted on release / document isolation")

    # Entering preferences again must start with saved shortcut values,
    # regardless of which navigation control dismissed the previous draft.
    pointer("Open " + first, offset=50)
    opened(first)
    for exit_path in ("cancel", "escape", "done", "sidebar-note", "sidebar-folder", "sidebar-settings"):
        pointer("Settings")
        pointer("Keyboard shortcuts")
        original = control("Shortcut New note", "textField").get("value", "")
        replace("Shortcut New note", "Cmd+Alt+9")
        if exit_path == "cancel":
            pointer("Cancel", "button")
        elif exit_path == "escape":
            key("Escape")
        elif exit_path == "done":
            pointer("Back from settings")
        elif exit_path == "sidebar-note":
            pointer("Open " + first, offset=50)
        elif exit_path == "sidebar-folder":
            pointer("Folder " + renamed, offset=50)
        else:
            pointer("Settings")
        assert settings().get("keys", {}).get("New note", "Cmd+N") == original
        pointer("Settings")
        pointer("Keyboard shortcuts")
        assert control("Shortcut New note", "textField").get("value", "") == original, exit_path
        pointer("Back from settings")
        pointer("Open " + first, offset=50)
        opened(first)
    check("shortcut draft cancellation through six exit paths / no persisted changes")

    pointer("Settings")
    pointer("Keyboard shortcuts")
    protected = {node["label"]: node["bounds"] for node in nodes()
                 if node.get("label") in {"Global quick note shortcut", "Save shortcuts", "Back from settings"}
                 or node.get("label", "").startswith(("Folder Journey", "Open Journey"))}
    assert len(protected) >= 6, protected
    for label in ("Folder " + renamed, "Open " + first, "Open " + second):
        for offset in (50, 200, 50):
            call("input", events=[event(control(label), "pointerMove", offset=offset)])
            actual = {node.get("label"): node["bounds"] for node in nodes() if node.get("label") in protected}
            assert actual == protected, (label, offset, protected, actual)
    pointer("Back from settings")
    check("native folder and note hover / action boundaries / stable settings geometry")

    # Cancel a bulk move without changing records, then leave selection mode.
    home()
    replace("Search notes", "Journey")
    pointer("Select notes")
    pointer("Select all")
    before = snapshot()
    pointer("Move selected…")
    replace("Folder name", "Must not be created")
    pointer("Cancel", "button")
    assert snapshot() == before, "Canceling batch move modified records"
    pointer("Cancel selection")
    assert not any(node.get("label") == "Move selected…" for node in nodes())
    assert "Must not be created" not in settings()["folders"]
    pointer("Note " + first)
    opened(first)
    check("search-scoped batch selection / move cancellation / ordinary click restored")

    # Child creation uses the hovered parent's action even when another note
    # was selected. Collapse and destructive actions must preserve that target.
    pointer("Open " + second, offset=50)
    opened(second)
    sidebar_menu(first)
    pointer("New sub-note")
    child = "Journey child"
    replace("Document title", child)
    key("Enter")
    call("input", events=[dict(type="text", text="Child body survives restore")])
    pointer("Open " + second, offset=50)
    opened(second)
    parent_record, child_record = stored(first), stored(child)
    assert child_record.get("parent") == parent_record["id"]
    assert child_record.get("folder") == renamed
    parent_row = control("Open " + first)
    call("input", events=[event(parent_row, "pointerMove", offset=50)])
    call("input", events=[event(control("Open " + first), offset=27)])
    opened(second)
    assert not any(node.get("label") == "Open " + child for node in nodes())
    call("input", events=[event(control("Open " + first), offset=27)])
    control("Open " + child)
    opened(second)
    check("hovered parent child creation / collapse / expand / selected note unchanged")

    sidebar_menu(first)
    pointer("Move to Trash")
    assert stored(first, "trash")["source"] == parent_record["source"]
    assert stored(child, "trash")["source"] == child_record["source"]
    assert not any(note["id"] in {parent_record["id"], child_record["id"]} for note in records())
    home()
    pointer("Trash", "button")
    act("Actions " + first)
    pointer("Restore note")
    assert stored(child).get("parent") == stored(first)["id"]
    assert stored(child)["source"] == child_record["source"]
    assert stored(first)["source"] == parent_record["source"]
    assert not any(note["id"] in {parent_record["id"], child_record["id"]} for note in records("trash"))
    pointer("Open " + child, offset=50)
    opened(child)
    assert control("Document editor").get("value") == "Child body survives restore"
    check("parent subtree trash / restore / hierarchy and exact content preserved")

    # Check a settings round trip in both languages without altering documents.
    before = snapshot()
    pointer("Settings")
    pointer("General")
    pointer("Application language")
    pointer("简体中文")
    pointer("键盘快捷键")
    for label in ("恢复默认值", "保存快捷键", "取消", "全局快速笔记快捷键"):
        target = control(label)
        assert "offscreen" not in target.get("states", [])
    pointer("通用")
    pointer("应用语言")
    pointer("English")
    pointer("Back from settings")
    opened(child)
    assert snapshot() == before, "Language round trip changed document records"
    check("Chinese and English settings / reachable controls / document isolation")

    # Delete a whole folder with pending edits in a child, then recover its tree.
    pointer("Folder " + renamed, offset=50, button="right")
    pointer("Delete folder…")
    pointer("Cancel", "button")
    assert snapshot() == before, "Canceling folder deletion changed records"
    opened(child)
    pointer("Document editor")
    key("Cmd+End")
    call("input", events=[dict(type="text", text=" saved before folder deletion")])
    pointer("Folder " + renamed, offset=50, button="right")
    pointer("Delete folder…")
    pointer("Delete folder", "button")
    control("Search notes")
    assert renamed not in settings().get("folders", [])
    assert not any(note.get("folder") == renamed for note in records())
    assert stored(child, "trash")["source"] == "Child body survives restore saved before folder deletion"
    assert stored(child, "trash")["trashRoot"] == stored(renamed, "trash")["id"]
    assert stored(second, "trash")["source"] == "Second body"
    assert not any(node.get("label") == "Folder " + renamed for node in nodes())
    pointer("Trash", "button")
    act("Actions " + renamed)
    pointer("Restore folder")
    assert stored(first).get("folder") == renamed
    assert renamed in settings()["folders"]
    assert stored(child).get("parent") == stored(first)["id"]
    pointer("Open " + child, offset=50)
    opened(child)
    assert control("Document editor").get("value") == "Child body survives restore saved before folder deletion"
    check("folder deletion cancellation / pending edits saved / folder trash entry / restore original hierarchy")
    home()
    return completed
