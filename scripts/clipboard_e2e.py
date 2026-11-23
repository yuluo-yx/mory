"""Exercise macOS pasteboard images and files through Mory's real paste menu."""
import json
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
