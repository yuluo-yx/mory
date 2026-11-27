"""Read native library documents for on-disk assertions in the macOS E2E suite."""
from xml.etree import ElementTree


def note_paths(directory):
    return sorted(path for path in directory.iterdir() if path.suffix.lower() in {".md", ".markdown", ".mdx", ".xml"} and path.is_file())


def read_note(path):
    data = path.read_bytes().decode("utf-8")
    source, separator, metadata = data.rpartition("\n\n<!-- mory:metadata:v1\n")
    assert separator and metadata.endswith("\n-->\n"), f"Missing document metadata: {path}"
    tree = ElementTree.fromstring(metadata[:-5])
    result = {child.tag: child.text or "" for child in tree}
    for key in {"pinned", "starred", "deleted", "hideInfo", "showCreated", "hideReferences"}:
        if key in result:
            result[key] = result[key] == "true"
    result["source"] = source
    return result
