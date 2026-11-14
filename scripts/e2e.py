#!/usr/bin/env python3
"""Exercise a real macOS MyGo window using its opt-in automation socket."""
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time
import struct
import zlib

from editor_e2e import exercise_editor


def exercise(binary, root):
    sock_path = root / "app.sock"
    fixtures = root / "fixtures"
    fixtures.mkdir()
    (fixtures / "Fixture import.md").write_text("# Imported heading\n\nA portable Markdown document.\n")
    (fixtures / "Personal.css").write_text(':root{--theme-name:"Personal"}.page{font-size:17px;color:#334433;background-color:#fffefa}h1{font-size:2em}')
    (fixtures / "collection").mkdir()
    (fixtures / "collection" / "Structured.xml").write_text('<title>Imported collection</title><figure view-type="Preview"><source path="image.png" name="Image attachment"/></figure><p>Local folder document</p><table><tr><td><p>One</p></td><td><p>Two</p></td></tr></table><latex>x^2</latex>')
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
    png = b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", 1, 1, 8, 6, 0, 0, 0))
    png += chunk(b"IDAT", zlib.compress(b"\0\x66\x7c\x54\xff")) + chunk(b"IEND", b"")
    (fixtures / "image.png").write_bytes(png)
    (fixtures / "collection" / "image.png").write_bytes(png)
    env = dict(os.environ, MORY_LIBRARY=str(root / "library"), MYGO_AUTOMATION=str(sock_path), MORY_E2E_PDF=str(root / "export.pdf"), MORY_E2E_FIXTURES=str(fixtures))
    process = subprocess.Popen([str(binary)], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    channel = None
    try:
        deadline = time.monotonic() + 20
        while not sock_path.exists():
            if process.poll() is not None:
                raise RuntimeError(process.communicate()[1].decode())
            if time.monotonic() > deadline:
                raise TimeoutError("Native app did not start")
            time.sleep(0.05)
        channel = socket.socket(socket.AF_UNIX)
        channel.settimeout(20)
        channel.connect(str(sock_path))
        stream = channel.makefile("rwb")

        def call(method, **params):
            stream.write((json.dumps(dict(id=1, method=method, params=params)) + "\n").encode())
            stream.flush()
            result = json.loads(stream.readline())
            if "error" in result:
                raise RuntimeError(result["error"])
            # Input is consumed during rendering. Force a native frame even
            # when the window is occluded before the next command reads or saves.
            if method in {"act", "input"}:
                call("capture")
            return result.get("result", {})

        def nodes():
            # Native surfaces may suspend their display link while occluded.
            # Capture builds an actual native frame before reading accessibility.
            call("capture")
            return call("tree")["nodes"]

        def act(label, action="press", **params):
            deadline = time.monotonic() + 5
            while True:
                tree = nodes()
                node = next((n for n in tree if n.get("label") == label and action in n.get("actions", [])), None)
                if node is not None or time.monotonic() >= deadline:
                    break
                time.sleep(0.05)
            if node is None:
                raise AssertionError(f"Missing {action} control: {label}; visible: {[n.get('label') for n in tree]}")
            call("act", id=node["id"], action=action, **params)
            # Tree reads the last completed frame; let the main loop render it.
            time.sleep(0.04)

        def key(name):
            call("input", events=[dict(type="key", key=name)])
            time.sleep(0.04)

        def wait_editor_text(label, text):
            # Input events are queued by the native surface. A capture alone can
            # precede their consumption; observe the editor before invoking Save.
            deadline = time.monotonic() + 5
            while True:
                field = next((n for n in nodes() if n.get("label") == label), {})
                if text in field.get("value", ""):
                    return
                if time.monotonic() >= deadline:
                    tree = call("tree")
                    focused = next((n for n in tree["nodes"] if n["id"] == tree.get("focus")), None)
                    raise AssertionError(f"Editor did not receive input: {label}: {field}; focused={focused}")
                time.sleep(0.05)

        windows = call("windows")["windows"]
        assert any(w["kind"] == "content" for w in windows), "Editor is not a native window"
        pdf_deadline = time.monotonic() + 25
        while not (root / "export.pdf").exists():
            if time.monotonic() > pdf_deadline:
                raise AssertionError("Native PDF export did not finish")
            time.sleep(0.1)
        assert (root / "export.pdf").read_bytes().startswith(b"%PDF-"), "Invalid PDF export"
        call("menu", path=["Help", "Open welcome guide"])
        welcome_deadline = time.monotonic() + 5
        while not any(n.get("label") == "Document title" and n.get("value") == "Welcome to Mory" for n in nodes()):
            if time.monotonic() > welcome_deadline:
                raise AssertionError("Welcome guide did not finish opening")
            time.sleep(0.05)
        guide = next(json.loads(p.read_text()) for p in (root / "library" / "notes").glob("*.json")
                     if json.loads(p.read_text())["title"] == "Welcome to Mory")
        assert guide["source"] == (Path(__file__).resolve().parent.parent / "resources" / "welcome.md").read_text()
        assert (root / "library" / "assets" / "mory-welcome.png").read_bytes().startswith(b"\x89PNG")
        call("menu", path=["File", "New note"])
        time.sleep(0.1)
        title = "Native E2E 世界 🌱"
        act("Document title", "setValue", text=title)
        act("Document editor", "focus")
        call("input", events=[dict(type="text", text="Hello 世界 — native editing")])
        wait_editor_text("Document editor", "Hello 世界 — native editing")
        key("Cmd+A")
        act("Bold")
        call("menu", path=["File", "Save"])
        time.sleep(0.1)
        notes = [json.loads(p.read_text()) for p in (root / "library" / "notes").glob("*.json")]
        note = next(n for n in notes if n["title"] == title)
        assert "**Hello 世界 — native editing**" in note["source"], note
        act("Source")
        act("Split")
        act("Read")
        act("Write")
        act("Plugins")
        act("Run Reading statistics")
        act("Toggle favorite")
        act("Toggle focus mode")
        act("Toggle focus mode")
        act("Document actions")
        act("Move to Trash")
        active_path = root / "library" / "notes" / (note["id"] + ".json")
        trash_path = root / "library" / "trash" / (note["id"] + ".json")
        assert not active_path.exists() and trash_path.exists(), "Delete did not move the library record"
        assert json.loads(trash_path.read_text()).get("deletedAt"), "Deletion time is missing"
        act("Trash")
        act("Actions " + title)
        assert not any(n.get("label") in {"Note name", "Rename", "Move to Trash"} for n in nodes()), "Trash exposed ordinary document details"
        act("Restore note")
        assert active_path.exists() and not trash_path.exists(), "Restore did not return the library record"
        act("Favorites")
        act("Note " + title)
        # A quote starts empty, receives authored text, and preserves its syntax.
        call("menu", path=["File", "New note"])
        act("Document title", "setValue", text="Native empty quote")
        act("Insert block")
        act("Insert Quote")
        call("menu", path=["File", "Save"])
        quote_path = next(p for p in (root / "library" / "notes").glob("*.json")
                          if json.loads(p.read_text())["title"] == "Native empty quote")
        assert json.loads(quote_path.read_text())["source"].strip() == ">", "Quote contains sample text"
        field = next(n for n in nodes() if n.get("label", "").startswith("Block text "))
        assert not field.get("value"), "Empty quote exposes authored content"
        box = field["bounds"]
        call("input", events=[dict(type="click", x=box["x"]+20, y=box["y"]+12)])
        call("input", events=[dict(type="text", text="My quotation 世界")])
        wait_editor_text(field["label"], "My quotation 世界")
        call("menu", path=["File", "Save"])
        nodes()
        assert "> My quotation 世界" in json.loads(quote_path.read_text())["source"]
        # Deleting the last slash-inserted callout must leave a live caret.
        call("menu", path=["File", "New note"])
        act("Document title", "setValue", text="Native block deletion")
        act("Document editor", "focus")
        call("input", events=[dict(type="text", text="/")])
        act("Insert Callout")
        act("Block actions callout")
        # Accessibility press does not move keyboard focus like a pointer click.
        act("Block actions callout", "focus")
        key("Cmd+A")
        key("Backspace")
        empty_field = next(n for n in nodes() if n.get("label", "").startswith("Block text "))
        assert not empty_field.get("value"), "Deleted callout still contains text"
        call("input", events=[dict(type="text", text="Continue")])
        wait_editor_text(empty_field["label"], "Continue")
        call("input", events=[dict(type="text", text=" after deletion 世界")])
        wait_editor_text(empty_field["label"], "Continue after deletion 世界")
        call("menu", path=["File", "Save"])
        nodes()
        after_delete = next(json.loads(p.read_text()) for p in (root / "library" / "notes").glob("*.json")
                            if json.loads(p.read_text())["title"] == "Native block deletion")
        assert "Continue after deletion 世界" in after_delete["source"] and "[!NOTE]" not in after_delete["source"], after_delete["source"]
        # Code editing and structural edits use one undo history, including
        # when keyboard shortcuts and the native Edit menu are mixed.
        call("menu", path=["File", "New note"])
        act("Document title", "setValue", text="Native code undo")
        act("Insert block")
        act("Insert Code block")
        act("Code editor", "focus")
        call("input", events=[dict(type="text", text="package main")])
        wait_editor_text("Code editor", "package main")
        key("Cmd+A")
        key("Backspace")
        key("Cmd+Z")
        wait_editor_text("Code editor", "package main")
        call("menu", path=["Edit", "Redo"])
        assert next(n.get("value", "") for n in nodes() if n.get("label") == "Code editor") == ""
        call("menu", path=["Edit", "Undo"])
        wait_editor_text("Code editor", "package main")
        key("Cmd+Shift+Z")
        assert next(n.get("value", "") for n in nodes() if n.get("label") == "Code editor") == ""
        call("menu", path=["Edit", "Undo"])
        call("menu", path=["Edit", "Undo"])
        call("menu", path=["Edit", "Undo"])
        assert not any(n.get("label") == "Code editor" for n in nodes()), "Empty code swallowed insertion undo"
        # Keyboard and mouse can both leave code without changing its contents.
        act("Insert block")
        act("Insert Code block")
        act("Code editor", "focus")
        call("input", events=[dict(type="text", text="package main")])
        wait_editor_text("Code editor", "package main")
        key("Down")
        prose = next(n for n in nodes() if n.get("label", "").startswith("Block text "))
        call("input", events=[dict(type="text", text="Outside")])
        wait_editor_text(prose["label"], "Outside")
        call("input", events=[dict(type="text", text=" code")])
        wait_editor_text(prose["label"], "Outside code")
        act("Write below code")
        fresh_prose = next(n for n in nodes() if n.get("label", "").startswith("Block text ") and not n.get("value"))
        call("input", events=[dict(type="text", text="Mouse paragraph")])
        wait_editor_text(fresh_prose["label"], "Mouse paragraph")
        call("menu", path=["File", "Save"])
        nodes()
        code_note = next(json.loads(p.read_text()) for p in (root / "library" / "notes").glob("*.json")
                         if json.loads(p.read_text())["title"] == "Native code undo")
        fenced_parts = code_note["source"].split("```")
        assert len(fenced_parts) == 3 and "package main" in fenced_parts[1] and "Mouse paragraph" in fenced_parts[2] and "Outside code" in fenced_parts[2], code_note["source"]
        exercise_editor(call, nodes, act, key, root, fixtures)
        act("All notes")
        act("Grid view")
        act("Note Native code undo")
        act("Settings")
        act("Editor")
        act("Larger text")
        act("Smaller text")
        assert not any(n.get("label") == "Document editor" for n in nodes()), "Settings is still an overlay"
        act("General")
        act("Application language")
        act("简体中文")
        assert any(n.get("label") == "应用语言" for n in nodes()), "Chinese interface did not activate"
        assert json.loads((root / "library" / "settings.json").read_text())["language"] == "zh-CN"
        act("应用语言")
        act("English")
        act("Keyboard shortcuts")
        act("Enable global quick note")
        act("Global quick note shortcut", "setValue", text="Ctrl+Alt+Shift+Space")
        act("Shortcut New note", "setValue", text="Cmd+Alt+J")
        act("Save shortcuts")
        saved_settings = json.loads((root / "library" / "settings.json").read_text())
        assert saved_settings["keys"]["New note"] == "Cmd+Alt+J"
        assert saved_settings["quickNote"] == "Ctrl+Alt+Shift+Space"
        act("Outline on Left")
        assert json.loads((root / "library" / "settings.json").read_text())["outlineSide"] == "Left"
        act("Outline on Right")
        # Clear the global hotkey again and verify that no Alt+Space fallback is saved.
        act("Settings")
        act("Keyboard shortcuts")
        act("Restore defaults")
        act("Save shortcuts")
        assert not json.loads((root / "library" / "settings.json").read_text()).get("quickNote")
        call("menu", path=["File", "Import…"])
        time.sleep(0.2)
        assert any(n.get("value") == "Fixture import" for n in nodes()), "Import did not open the document"
        act("Insert block")
        act("Image or attachment…")
        time.sleep(0.2)
        for format_name, extension in [("Markdown", "md"), ("DocxXML", "xml"), ("HTML", "html"), ("PDF", "pdf")]:
            act("Export note")
            act("Export " + format_name)
            exported = fixtures / ("Fixture import." + extension)
            deadline = time.monotonic() + 25
            while not exported.exists() or (extension == "md" and "assets/" not in exported.read_text()):
                if time.monotonic() > deadline:
                    raise AssertionError("Export did not finish: " + extension)
                time.sleep(0.1)
        assert "data:image/" in (fixtures / "Fixture import.html").read_text()
        assert (fixtures / "Fixture import.pdf").read_bytes().startswith(b"%PDF-")
        act("Settings")
        act("Themes")
        act("Import CSS…")
        act("Apply stylesheet")
        act("Document actions")
        act("Choose cover…")
        deadline = time.monotonic() + 5
        while not any(n.get("label") == "Remove cover" for n in nodes()):
            if time.monotonic() > deadline:
                raise AssertionError("Cover import did not finish")
            time.sleep(0.05)
        act("Remove cover")
        act("Rename")
        call("menu", path=["File", "Save"])
        call("menu", path=["File", "Open local file"])
        act("Document title", "setValue", text="Original file editing")
        act("Source")
        # This path verifies saving to the original file. Set the source through
        # its native accessibility action so window focus cannot swallow input;
        # native keyboard editing is exercised separately above.
        source = next(n["value"] for n in nodes() if n.get("label") == "Source editor")
        act("Source editor", "setValue", text=source + "\nSaved to original file.")
        wait_editor_text("Source editor", "Saved to original file.")
        call("menu", path=["File", "Save"])
        nodes()
        assert "Saved to original file." in (fixtures / "Fixture import.md").read_text(), "Original file not updated"
        original_bytes = (fixtures / "Fixture import.md").read_bytes()
        act("Document actions")
        act("Remove from list")
        assert (fixtures / "Fixture import.md").read_bytes() == original_bytes, "Remove from list changed original"
        assert not any(json.loads(p.read_text()).get("external") for p in (root / "library" / "trash").glob("*.json"))
        act("▱   Import folder")
        act("Document title", "setValue", text="Structured import")
        deadline = time.monotonic() + 15
        while not any(n.get("label") == "Attachment thumbnail Image attachment" for n in nodes()):
            if time.monotonic() > deadline:
                raise AssertionError("Quick Look thumbnail did not finish")
            time.sleep(0.05)
        call("menu", path=["File", "Save"])
        call("menu", path=["File", "Print"])
        deadline = time.monotonic() + 25
        while not (fixtures / "print.pdf").exists():
            if time.monotonic() > deadline:
                raise AssertionError("Native print renderer did not finish")
            time.sleep(0.05)
        assert (fixtures / "print.pdf").read_bytes().startswith(b"%PDF-")
        # Static Docusaurus components render in the actual native window and
        # their view state must not rewrite front matter or MDX wrappers.
        docusaurus = ('---\ntitle: Native guide\n---\n\n# Native guide {#guide}\n\n'
                      ':::tip Remember\nPreserve this text.\n:::\n\n'
                      '<details><summary>More</summary>\n\nHidden detail.\n\n</details>\n\n'
                      '<Tabs><TabItem value="a" label="First">First body.</TabItem>\n'
                      '<TabItem value="b" label="Second">Second body.</TabItem></Tabs>\n')
        (fixtures / "Fixture import.md").write_text(docusaurus)
        call("menu", path=["File", "Import…"])
        act("Document title", "setValue", text="Native Docusaurus")
        act("Write")
        act("Settings")
        act("General")
        assert any(n.get("label") == "Make Mory the default Markdown app" for n in nodes())
        act("Themes")
        act("Writing style")
        act("Docusaurus")
        act("Back from settings")
        act("Toggle details More")
        assert any(n.get("value", "").strip() == "Hidden detail." for n in nodes())
        act("Tab Second")
        assert any(n.get("value", "").strip() == "Second body." for n in nodes())
        call("menu", path=["File", "Save"])
        native_guide = next(json.loads(p.read_text()) for p in (root / "library" / "notes").glob("*.json")
                            if json.loads(p.read_text())["title"] == "Native Docusaurus")
        assert native_guide["source"] == docusaurus, "Docusaurus preview changed source"
        # Independent windows keep separate edit buffers and version stamps.
        call("menu", path=["File", "Open in new window"])
        time.sleep(0.15)
        window_list = call("windows")["windows"]
        assert len([w for w in window_list if w["kind"] == "content"]) >= 2
        call("menu", path=["File", "Quick note"])
        time.sleep(0.15)
        assert any(w["title"] == "Mory Quick Note" for w in call("windows")["windows"])
        call("menu", path=["File", "Quick note"])
        call("menu", path=["File", "Save"])
        call("menu", path=["Mory", "Quit Mory"])
        process.wait(timeout=10)
        assert process.returncode == 0, process.communicate()[1].decode()
        saved = json.loads((root / "library" / "notes" / (note["id"] + ".json")).read_text())
        assert not saved.get("deleted", False) and saved["starred"]
        print("Native E2E passed: launch, Unicode editing, formatting, save, modes, plugin, favorite, trash file moves, restore, empty quote editing, external removal, settings, native PDF, quit.")
    finally:
        if channel:
            channel.close()
        if process.poll() is None:
            process.terminate()
            process.wait(timeout=10)
        diagnostic = process.communicate()[1].decode()
        if diagnostic:
            print(diagnostic, file=sys.stderr)


if __name__ == "__main__":
    binary = Path(sys.argv[1]).resolve()
    output = Path(".test-output")
    output.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="e2e-", dir=output) as folder:
        exercise(binary, Path(folder).resolve())
