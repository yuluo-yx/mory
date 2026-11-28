"""Exercise macOS pasteboard images and files through Mory's real paste menu."""
import hashlib
import json
import struct
import zlib
import re
import subprocess
import time

from library_records import note_paths, read_note


def exercise_clipboard(call, nodes, act, key, root, fixtures):
    screenshot = fixtures / "Screenshot.tiff"
    subprocess.run(["sips", "-s", "format", "tiff", str(fixtures / "image.png"), "--out", str(screenshot)],
                   check=True, stdout=subprocess.DEVNULL)
    second = fixtures / "Second image.png"
    second.write_bytes((fixtures / "image.png").read_bytes())

    def clipboard(script):
        return subprocess.run(["osascript", "-e", script], check=True, capture_output=True, text=True).stdout.strip()

    def saved(title):
        call("menu", path=["File", "Save"])
        nodes()
        return next(read_note(p) for p in note_paths(root / "library" / "notes") if read_note(p)["title"] == title)

    for kind in ["PNG", "TIFF", "files"]:
        title = "Native clipboard " + kind
        call("menu", path=["File", "New note"])
        act("Document title", "setValue", text=title)
        act("Document editor", "focus")
        call("input", events=[dict(type="text", text="Before image")])
        nodes()
        if kind == "files":
            paths = [fixtures / "image.png", second]
            # Finder offers NSURL objects. AppleScript's plain alias list uses
            # its private serialized list format, which is not a file pasteboard.
            count = clipboard('\n'.join([
                'use framework "AppKit"',
                "set board to current application's NSPasteboard's generalPasteboard()",
                "set urls to current application's NSMutableArray's array()",
                *["urls's addObject:(current application's NSURL's fileURLWithPath:" + json.dumps(str(p)) + ")" for p in paths],
                "board's clearContents()",
                "board's writeObjects:urls",
                "return (board's pasteboardItems()'s |count|()) as integer",
            ]))
            assert count == str(len(paths)), ("Incomplete native file clipboard", count)
        else:
            path, native_type = (screenshot, "TIFF") if kind == "TIFF" else (fixtures / "image.png", "PNGf")
            clipboard("set the clipboard to (read POSIX file " + json.dumps(str(path)) + " as «class " + native_type + "»)")
        call("menu", path=["Edit", "Paste"])
        # Observe the completed native edit before saving; queued window updates
        # and image layout can span more than one frame on a busy CI desktop.
        expected_captions = {p.name for p in paths} if kind == "files" else {"Clipboard.png"}
        deadline = time.monotonic() + 5
        while not expected_captions.issubset({n.get("label") for n in nodes()}):
            if time.monotonic() >= deadline:
                raise AssertionError(("Native paste did not insert every image", kind, saved(title)["source"]))
            time.sleep(0.05)
        note = saved(title)
        images = re.findall(r"!\[[^\]]*\]\((assets/[^)]+)\)", note["source"])
        assert len(images) == (2 if kind == "files" else 1), (kind, note["source"])
        assert "Before image" in note["source"], note["source"]
        for image in images:
            assert (root / "library" / image).read_bytes().startswith(b"\x89PNG"), image
        act("All notes")
        act("Note " + title)
        assert saved(title)["source"] == note["source"], "Reopening lost clipboard content"

    # The custom native menu must still dispatch text to the focused responder.
    call("menu", path=["File", "New note"])
    clipboard('set the clipboard to "Native pasted title"')
    act("Document title", "focus")
    key("Cmd+A")
    call("menu", path=["Edit", "Paste"])
    assert any(n.get("label") == "Document title" and n.get("value") == "Native pasted title" for n in nodes())
    act("Document editor", "focus")
    clipboard('set the clipboard to "Native pasted body"')
    call("menu", path=["Edit", "Paste"])
    assert "Native pasted body" in saved("Native pasted title")["source"]
    print("Native clipboard passed: PNG, TIFF screenshot, multiple image files, save/reopen, title and body text paste.", flush=True)


def exercise_attachments(call, nodes, act, key, root, fixtures):
    image = fixtures / "image.png"
    original = image.read_bytes()
    payload = b"tEXtComment\0" + b"x" * (33 << 20)
    large = original[:-12] + struct.pack(">I", len(payload) - 4) + payload + struct.pack(">I", zlib.crc32(payload)) + original[-12:]
    title = "Native large attachment"

    def saved():
        call("menu", path=["File", "Save"])
        nodes()
        return next(read_note(p) for p in note_paths(root / "library" / "notes") if read_note(p)["title"] == title)

    def wait_label(label):
        deadline = time.monotonic() + 10
        while not any(n.get("label") == label for n in nodes()):
            if time.monotonic() > deadline:
                raise AssertionError("Missing native attachment result: " + label)
            time.sleep(0.05)

    try:
        image.write_bytes(large)
        call("menu", path=["File", "New note"])
        act("Document title", "setValue", text=title)
        act("Document editor", "focus")
        call("input", events=[dict(type="text", text="Keep the original paragraph")])
        before = saved()["source"]
        act("Insert block")
        act("Image or attachment…")
        wait_label("image.png")
        inserted = saved()
        asset = re.search(r"\]\((assets/[^)]+)\)", inserted["source"])[1]
        assert hashlib.sha256((root / "library" / asset).read_bytes()).digest() == hashlib.sha256(large).digest()
        key("Cmd+Z")
        undone = saved()["source"]
        assert undone == before, ("Attachment undo changed the original paragraph", before, undone, call("tree").get("focus"))
        key("Cmd+Shift+Z")
        assert saved()["source"] == inserted["source"], "Attachment redo lost its image"
        act("All notes")
        act("Note " + title)
        assert saved()["source"] == inserted["source"], "Reopening lost the large image"

        act("Document actions")
        act("Choose cover…")
        act("Upload")
        act("Choose local image…")
        wait_label("Remove cover")
        key("Escape")
        assert saved()["cover"] == asset, "Cover did not share the local asset limit"

        # A sparse file reaches the error branch without reading 129 MiB.
        with image.open("wb") as stream:
            stream.truncate(129 << 20)
        act("Insert block")
        act("Image or attachment…")
        wait_label("Could not add attachment")
        text = " ".join(str(n.get("value", "")) + " " + str(n.get("label", "")) for n in nodes())
        assert "129.0 MiB" in text and "128 MiB" in text and "Resize or compress" in text, text
        assert "Changes not saved" not in text and "Export a copy before closing" not in text, text
        act("Dismiss")
        assert saved()["source"] == inserted["source"], "Failed attachment changed document contents"

        image.write_bytes(original)
        act("Insert block")
        act("Image or attachment…")
        deadline = time.monotonic() + 10
        while saved()["source"].count("](assets/") != 2:
            if time.monotonic() > deadline:
                raise AssertionError("Attachment insertion remained blocked after dismissing the size error")
            time.sleep(0.05)
        # Presets use normal local PNGs and remain portable after reopening.
        act("Document actions")
        act("Choose cover…")
        act("Cover: Blue mountains")
        preset = saved()["cover"]
        assert preset != asset and (root / "library" / preset).read_bytes().startswith(b"\x89PNG")
        act("All notes")
        act("Note " + title)
        assert saved()["cover"] == preset
        cover = next(n["bounds"] for n in nodes() if n.get("label") == "Document cover")
        header = next(n["bounds"] for n in nodes() if n.get("label") == "Document header")
        assert abs(cover["x"] - header["x"]) < 1 and abs(cover["width"] - header["width"]) < 1, (cover, header)
        act("Source")
        long_source = "\n\n".join("## Section %d\n\nParagraph %d." % (i, i) for i in range(70))
        act("Source editor", "setValue", text=long_source)
        saved()
        act("Write")
        editor = next(n["bounds"] for n in nodes() if n.get("label") == "Document editor")
        title_area = next(n["bounds"] for n in nodes() if n.get("label") == "Document title area")
        call("input", events=[dict(type="wheel", x=editor["x"]+80, y=title_area["y"]+title_area["height"]+30, dy=750, precise=True)])
        assert not any(n.get("label") == "Document title" and "offscreen" not in n.get("states", []) for n in nodes()), "Cover/title stayed fixed over the editor"
        call("input", events=[dict(type="wheel", x=editor["x"]+80, y=editor["y"]+100, dy=-100000, precise=True)])
        wait_label("Document cover")
        assert saved()["source"] == long_source, "Cover scrolling changed the document"
        print("Native attachments passed: >32 MiB image/cover, byte preservation, undo/redo, reopen, size explanation, retry, full-width gallery and page scrolling.", flush=True)
    finally:
        image.write_bytes(original)
