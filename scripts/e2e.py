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

        windows = call("windows")["windows"]
        assert any(w["kind"] == "content" for w in windows), "Editor is not a native window"
        pdf_deadline = time.monotonic() + 25
        while not (root / "export.pdf").exists():
            if time.monotonic() > pdf_deadline:
                raise AssertionError("Native PDF export did not finish")
            time.sleep(0.1)
        assert (root / "export.pdf").read_bytes().startswith(b"%PDF-"), "Invalid PDF export"
        call("menu", path=["File", "New note"])
        time.sleep(0.1)
        title = "Native E2E 世界 🌱"
        act("Document title", "setValue", text=title)
        act("Document editor", "focus")
        call("input", events=[dict(type="text", text="Hello 世界 — native editing")])
        time.sleep(0.1)
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
        act("Trash")
        act("Note " + title)
        act("Restore note")
        act("Favorites")
        act("Note " + title)
        act("Settings")
        act("Larger text")
        act("Smaller text")
        act("Keyboard shortcuts")
        act("Done")
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
        act("Source editor", "focus")
        key("Cmd+End")
        call("input", events=[dict(type="text", text="\nSaved to original file.")])
        call("menu", path=["File", "Save"])
        nodes()
        assert "Saved to original file." in (fixtures / "Fixture import.md").read_text(), "Original file not updated"
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
        print("Native E2E passed: launch, Unicode editing, formatting, save, modes, plugin, favorite, trash, restore, settings, native PDF, quit.")
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
