"""User journeys through the real library, sidebar, settings and saved records.

Locate controls through accessibility, then use pointer and keyboard input for
the workflow under test. Never save or repair focus before checking a transition.
"""
import json
import time

from library_records import note_paths, read_note


def exercise_workspace(call, nodes, act, key, root, restart):
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

    # Single-line dialog input must submit the same action as its primary button.
    # Send text and Enter together to catch stale bound values in native batches.
    keyboard_folder = "Keyboard Nvidia 网络"
    pointer("New folder", "button")
    call("input", events=[dict(type="text", text=keyboard_folder), dict(type="key", key="Enter"),
                           dict(type="key", key="Enter")])
    wait("Enter did not close new folder dialog", lambda tree: not any(n.get("label") == "Folder title" for n in tree))
    assert settings()["folders"].count(keyboard_folder) == 1
    pointer("Folder " + keyboard_folder, offset=50)
    control("Search notes")
    new_note("Keyboard details original", "Preserved form body")
    sidebar_menu("Keyboard details original")
    pointer("Rename…")
    control("Note name", "textField")
    call("input", events=[dict(type="key", key="Cmd+A"), dict(type="text", text="Keyboard details renamed"),
                           dict(type="key", key="Enter")])
    wait("Enter did not close note details", lambda tree: not any(n.get("label") == "Note name" for n in tree))
    assert stored("Keyboard details renamed")["source"] == "Preserved form body"
    sidebar_menu("Keyboard details renamed")
    pointer("Move…")
    pointer("Folder name", "textField")
    call("input", events=[dict(type="key", key="Cmd+A"), dict(type="text", text="Keyboard moved folder"),
                           dict(type="key", key="Enter")])
    wait("Enter did not close move dialog", lambda tree: not any(n.get("label") == "Folder name" for n in tree))
    moved = stored("Keyboard details renamed")
    assert moved["folder"] == "Keyboard moved folder" and moved["source"] == "Preserved form body"
    expected_location = "/Keyboard moved folder/" + moved["fileName"]
    assert control("Document location").get("value") == expected_location

    def header_geometry():
        header = control("Document header")["bounds"]
        location = control("Document location")["bounds"]
        actions = control("Document header actions")["bounds"]
        last = control("Document actions")["bounds"]
        assert abs(last["x"]+last["width"]-(header["x"]+header["width"]-20)) <= 1, (header, last)
        assert location["height"] <= 24 and location["x"]+location["width"] <= actions["x"]-9, (location, actions)
        assert control("Document location").get("value") == expected_location
        return header["width"]

    normal_width = header_geometry()
    pointer("Toggle sidebar")
    header_geometry()
    # Exercise the real native full-screen transition, beyond wide-window
    # geometry in unit tests. Restore it before other desktop scenarios run.
    call("menu", path=["View", "Toggle Full Screen"])
    fullscreen_state = root / "fixtures" / "fullscreen-state"
    wait("Native full-screen entry did not finish", lambda tree: fullscreen_state.exists() and fullscreen_state.read_text() == "entered")
    wait("Window did not enter full screen", lambda tree: next((node for node in tree
         if node.get("label") == "Document header" and node["bounds"]["width"] > normal_width+400), None))
    header_geometry()
    call("menu", path=["View", "Toggle Full Screen"])
    wait("Native full-screen exit did not finish", lambda tree: fullscreen_state.read_text() == "left")
    wait("Window did not leave full screen", lambda tree: next((node for node in tree
         if node.get("label") == "Document header" and node["bounds"]["width"] < normal_width+400), None))
    header_geometry()
    pointer("Toggle sidebar")
    header_geometry()
    check("header right alignment / logical path / sidebar toggle / native full screen / no wrapping")
    home()
    pointer("New folder", "button")
    call("input", events=[dict(type="text", text=keyboard_folder), dict(type="key", key="Enter")])
    control("Dismiss")
    assert settings()["folders"].count(keyboard_folder) == 1
    pointer("Dismiss")
    key("Escape")
    check("form Enter / batched Unicode input / folder create / note rename / move / validation / no duplicates")

    # Context and more menus address physical files without opening another
    # editor. Verify the real clipboard by pasting into an ordinary UI field.
    def copied_path(title, expected, more=False):
        if more:
            sidebar_menu(title)
        else:
            pointer("Open " + title, offset=50, button="right")
        pointer("Copy absolute path")
        pointer("Search notes", "textField")
        key("Cmd+A")
        key("Cmd+V")
        assert control("Search notes", "textField").get("value") == str(expected)
        replace("Search notes", "")

    def finder_file(title, expected, more=False):
        if more:
            sidebar_menu(title)
        else:
            pointer("Open " + title, offset=50, button="right")
        pointer("Show in Finder")
        log = root / "fixtures" / "finder-selection.txt"
        wait("Finder received the wrong file", lambda _: log.exists() and log.read_text() == str(expected))

    path_title = "Keyboard details renamed"
    physical = next(path for path in note_paths(library / "notes") if read_note(path)["title"] == path_title)
    assert physical.name == path_title + ".md", physical
    original = physical.read_bytes()
    copied_path(path_title, physical)
    finder_file(path_title, physical, more=True)
    assert physical.read_bytes() == original

    # An empty logical folder opens shared storage, never an invented path
    # based on the display title. Cover both menu entry points independently.
    for more in (False, True):
        log = root / "fixtures" / "finder-folder.txt"
        log.unlink(missing_ok=True)
        label = "Folder " + keyboard_folder
        if more:
            row = control(label)
            call("input", events=[event(row, "pointerMove", offset=50)])
            row = control(label)
            call("input", events=[event(row, offset=row["bounds"]["width"] - 15)])
        else:
            pointer(label, offset=50, button="right")
        pointer("Open storage folder in Finder")
        wait("Finder received a virtual folder path", lambda _: log.exists() and log.read_text() == str(library / "notes"))

    call("menu", path=["File", "Open local file"])
    opened("Fixture import")
    home()
    external = (root / "fixtures" / "Fixture import.md").resolve()
    original = external.read_bytes()
    copied_path("Fixture import", external, more=True)
    finder_file("Fixture import", external)
    assert external.read_bytes() == original
    sidebar_menu("Fixture import")
    pointer("Remove from list")
    assert external.read_bytes() == original
    check("path clipboard / Finder selection / internal and external files / empty folder context and more menus")

    # Import copies retain the original basename, independently of the display
    # title, while repeated imports allocate distinct, readable destinations.
    for expected in ("Fixture import.md", "Fixture import (2).md"):
        call("menu", path=["File", "Import…"])
        opened("Fixture import")
        home()
        imported = library / "notes" / expected
        wait("Import did not retain its basename", lambda _: imported.exists())
        assert read_note(imported)["fileName"] == expected
        assert read_note(imported)["source"] == external.read_text()
    for expected in ("Fixture import.md", "Fixture import (2).md"):
        sidebar_menu("Fixture import")
        pointer("Move to Trash")
    for expected in ("Fixture import.md", "Fixture import (2).md"):
        assert (library / "trash" / expected).exists()
        assert not (library / "notes" / expected).exists()
    check("original import basename / duplicate suffix / renamed document filename / unchanged original")

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

    # Preferences replace the document tree, then restore its navigation.
    pointer("Settings")
    pointer("Keyboard shortcuts")
    assert not any(node.get("label", "").startswith(("Folder ", "Open Journey")) for node in nodes())
    pointer("Back from settings")
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
    for exit_path in ("cancel", "escape", "back"):
        pointer("Settings")
        pointer("Keyboard shortcuts")
        original = control("Shortcut New note", "textField").get("value", "")
        replace("Shortcut New note", "Cmd+Alt+9")
        if exit_path == "cancel":
            pointer("Cancel", "button")
        elif exit_path == "escape":
            key("Escape")
        else:
            pointer("Back from settings")
        assert settings().get("keys", {}).get("New note", "Cmd+N") == original
        pointer("Settings")
        pointer("Keyboard shortcuts")
        assert control("Shortcut New note", "textField").get("value", "") == original, exit_path
        pointer("Back from settings")
        pointer("Open " + first, offset=50)
        opened(first)
    check("shortcut draft cancellation through all settings exit paths / no persisted changes")

    pointer("Settings")
    pointer("Keyboard shortcuts")
    categories = ("General", "Editor", "Themes", "Keyboard shortcuts", "About")
    protected = {node["label"]: node["bounds"] for node in nodes()
                 if node.get("label") in {"Global quick note shortcut", "Save shortcuts", "Back from settings", *categories}}
    assert len(protected) == 8, protected
    for label in categories:
        for offset in (50, 180, 50):
            call("input", events=[event(control(label), "pointerMove", offset=offset)])
            actual = {node.get("label"): node["bounds"] for node in nodes() if node.get("label") in protected}
            assert actual == protected, (label, offset, protected, actual)
    for label, expected in (("General", "Application language"), ("Editor", "Outline text size"),
                            ("Themes", "Writing style"), ("About", "Application version")):
        pointer(label, "tab")
        control(expected)
        title = control("Settings section title")["bounds"]
        tab = control(label, "tab")
        assert tab["bounds"]["x"] + tab["bounds"]["width"] < title["x"], (label, tab, title)
        # The native automation bridge exposes the selected tab as checked.
        assert "checked" in tab.get("states", []), tab
        assert not any(node.get("label", "").startswith(("Folder ", "Open Journey")) for node in nodes())
    control("Project on GitHub")
    key("Home")
    control("Application language")
    key("Down")
    control("Outline text size")
    key("End")
    control("Application version")
    pointer("Back from settings")
    opened(first)
    check("settings sidebar replacement / five categories / stable hover / keyboard navigation / about / return to document")

    # A fresh library has one built-in style. User CSS remains switchable and
    # survives a reset; presentation changes must never alter document source.
    before = snapshot()
    pointer("Settings")
    pointer("Themes")
    assert control("Writing style").get("value") == "yuluo"
    pointer("Edit CSS")
    assert control("Theme name", "textField").get("value") == "yuluo"
    replace("Theme name", "Journey custom")
    css = '.page { font-size: 18px; color: #234567; } a { color: #0969da; }'
    pointer("Theme CSS")
    call("input", events=[dict(type="key", key="Cmd+A"), dict(type="text", text=css)])
    pointer("Apply stylesheet")
    custom_file = library / "themes" / "journey custom.css"
    assert custom_file.read_text() == css
    assert settings()["theme"] == "Journey custom"
    pointer("Settings")
    pointer("Writing style")
    pointer("yuluo")
    assert settings()["theme"] == "yuluo"
    pointer("Writing style")
    pointer("journey custom")
    assert settings()["theme"] == "journey custom"
    pointer("Reset theme")
    assert settings()["theme"] == "yuluo" and custom_file.read_text() == css
    pointer("Back from settings")
    opened(first)
    assert snapshot() == before, "Theme changes rewrote document records"
    check("single yuluo theme / edit CSS / saved custom style / switching / non-destructive reset / document preservation")

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

    # Relaunch the actual process. Only the most recently used document's
    # folder is expanded, regardless of other manually opened folders.
    before = snapshot()
    restart()
    control("Search notes")
    for name in settings()["folders"]:
        folder_node = control("Folder " + name)
        assert ("expanded" in folder_node.get("states", [])) == (name == renamed), folder_node
    control("Open " + child)
    assert not any(node.get("label") == "Open Keyboard details renamed" for node in nodes())
    assert snapshot() == before, "Restart changed document records"
    pointer("Folder Keyboard moved folder", offset=50)
    pointer("Note Keyboard details renamed")
    opened("Keyboard details renamed")
    control("Open Keyboard details renamed")
    restart()
    for name in settings()["folders"]:
        folder_node = control("Folder " + name)
        assert ("expanded" in folder_node.get("states", [])) == (name == "Keyboard moved folder"), folder_node
    assert not any(node.get("label") == "Open " + child for node in nodes())
    assert snapshot() == before, "Opening or relaunching changed document contents"
    check("native quit and restart / only last document folder expanded / nested child visible / other folders collapsed")

    # Home contains folders and root documents; opening the paper preview is a
    # navigation click, never an embedded link, checkbox or editable selection.
    before = snapshot()
    control("Open folder " + renamed)
    assert not any(node.get("label") == "Note " + first for node in nodes())
    pointer("Open folder " + renamed)
    card = control("Note " + first)["bounds"]
    assert card["height"] > card["width"] and card["width"] < 290, card
    pointer("Note " + first)
    opened(first)
    pointer("Back to library")
    home()
    pointer("Open folder " + renamed)
    pointer("List view")
    control("Note " + first)
    pointer("Parent folder")
    control("Open folder " + renamed)
    pointer("Grid view")
    replace("Search notes", first)
    control("Note " + first)
    assert not any(node.get("label", "").startswith("Open folder ") for node in nodes())
    replace("Search notes", "")
    assert snapshot() == before, "Home previews or navigation modified document contents"
    check("home folder cards / portrait paper preview / folder navigation / parent / grid-list / nested search / source preservation")
    return completed
