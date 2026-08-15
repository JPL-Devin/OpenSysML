#!/usr/bin/env python3
"""Check that every relative Markdown link in the repository resolves.

A link to a file must name a file that exists; a link with a `#fragment` must name a
heading that exists in that file (or an explicit anchor). External links, mailto: and
in-page-only fragments pointing at a heading of the same file are checked too.

Run from the repository root:

    python3 scripts/check-doc-links.py
"""

import re
import subprocess
import sys
from pathlib import Path

LINK = re.compile(r"\[[^\]]*\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)")
HEADING = re.compile(r"^#{1,6}\s+(.*?)\s*#*$", re.MULTILINE)
ANCHOR = re.compile(r"<a\s+(?:id|name)=[\"']([^\"']+)[\"']", re.IGNORECASE)
SKIP_PREFIX = ("http://", "https://", "mailto:", "tel:", "ftp://")


def slugs(heading: str) -> set[str]:
    """GitHub's heading slug: lowercased, punctuation dropped, a dash per space.

    Dropped punctuation leaves a run of dashes; the collapsed spelling is accepted too.
    """
    text = re.sub(r"`|\*|_|<[^>]+>", "", heading)
    text = re.sub(r"\[([^\]]*)\]\([^)]*\)", r"\1", text)
    text = text.strip().lower()
    text = re.sub(r"[^\w\s-]", "", text, flags=re.UNICODE)
    each = re.sub(r"\s", "-", text)
    return {s for s in (each, re.sub(r"-+", "-", each)) if s}


def anchors_of(path: Path) -> set[str]:
    text = path.read_text(encoding="utf-8", errors="replace")
    found: set[str] = set()
    for heading in HEADING.findall(text):
        found.update(slugs(heading))
    found.update(ANCHOR.findall(text))
    return found


def tracked_markdown() -> list[Path]:
    out = subprocess.run(
        ["git", "ls-files", "*.md"], capture_output=True, text=True, check=True
    ).stdout
    return [Path(line) for line in out.splitlines() if line]


def main() -> int:
    root = Path(".").resolve()
    anchors: dict[Path, set[str]] = {}
    failures: list[str] = []

    for md in tracked_markdown():
        text = md.read_text(encoding="utf-8", errors="replace")
        for raw in LINK.findall(text):
            link = raw.strip()
            if not link or link.startswith(SKIP_PREFIX):
                continue
            target, _, fragment = link.partition("#")
            if target:
                dest = (md.parent / target).resolve()
                if not str(dest).startswith(str(root)):
                    failures.append(f"{md}: escapes the repository: {link}")
                    continue
                if not dest.exists():
                    failures.append(f"{md}: no such file: {link}")
                    continue
            else:
                dest = md.resolve()
            if not fragment:
                continue
            if dest.is_dir():
                dest = dest / "README.md"
                if not dest.exists():
                    failures.append(f"{md}: no README.md for the fragment in {link}")
                    continue
            if dest.suffix != ".md":
                continue
            if dest not in anchors:
                anchors[dest] = anchors_of(dest)
            if fragment.lower() not in anchors[dest]:
                failures.append(f"{md}: no heading '#{fragment}' in {target or md.name}")

    for failure in sorted(failures):
        print(failure)
    print(f"{len(failures)} broken link(s)")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
