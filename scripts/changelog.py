#!/usr/bin/env python3
"""Keep CHANGELOG.md out of every pull request.

A change describes itself in a fragment under changes/unreleased/ instead of
editing the shared "## Unreleased" section, so two branches that both add an
entry never touch the same lines. The fragments are folded into CHANGELOG.md
when a release is prepared.

    changes/unreleased/<slug>.<section>.md

<slug> is free-form (the branch or topic name); <section> is one of the
Keep a Changelog headings this file uses, lower-cased: added, changed,
deprecated, removed, fixed, security, performance. The body is the entry as it
will appear — one or more list items in the changelog's own style.

Run from the repository root:

    python3 scripts/changelog.py check              # every fragment is well-formed (CI)
    python3 scripts/changelog.py render             # fold fragments into "## Unreleased", delete them
    python3 scripts/changelog.py release 0.5.0      # render, then date the section as a release
    python3 scripts/changelog.py release 0.5.0 --date 2026-09-10
"""

from __future__ import annotations

import argparse
import datetime as _dt
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
CHANGELOG = ROOT / "CHANGELOG.md"
FRAGMENTS = ROOT / "changes" / "unreleased"

SECTIONS = ["Added", "Changed", "Deprecated", "Removed", "Fixed", "Security", "Performance"]
SECTION_BY_KEY = {s.lower(): s for s in SECTIONS}

FRAGMENT_NAME = re.compile(r"^(?P<slug>[A-Za-z0-9][A-Za-z0-9._-]*)\.(?P<section>[a-z]+)\.md$")
UNRELEASED = re.compile(r"^## Unreleased[ \t]*\n", re.MULTILINE)
VERSION_HEADING = re.compile(r"^## (?!Unreleased)", re.MULTILINE)
SECTION_HEADING = re.compile(r"^### (?P<name>.+?)\s*$", re.MULTILINE)


class FragmentError(Exception):
    pass


def _rel(path: pathlib.Path) -> str:
    try:
        return str(path.relative_to(ROOT))
    except ValueError:
        return str(path)


def fragments() -> list[pathlib.Path]:
    if not FRAGMENTS.is_dir():
        return []
    return sorted(p for p in FRAGMENTS.iterdir() if p.is_file() and p.name != "README.md" and not p.name.startswith("."))


def parse_fragment(path: pathlib.Path) -> tuple[str, str]:
    """Return (section, body) for one fragment, or raise FragmentError."""
    m = FRAGMENT_NAME.match(path.name)
    if not m:
        raise FragmentError(f"{_rel(path)}: name must be <slug>.<section>.md")
    key = m.group("section")
    if key not in SECTION_BY_KEY:
        raise FragmentError(
            f"{_rel(path)}: section {key!r} is not one of {', '.join(SECTION_BY_KEY)}"
        )
    body = path.read_text(encoding="utf-8").strip("\n")
    if not body.strip():
        raise FragmentError(f"{_rel(path)}: fragment is empty")
    for line in body.splitlines():
        if line.startswith("#"):
            raise FragmentError(
                f"{_rel(path)}: fragments hold list items only; the heading comes from the file name"
            )
    if not body.lstrip().startswith("- "):
        raise FragmentError(f"{_rel(path)}: body must start with a list item ('- ')")
    return SECTION_BY_KEY[key], body


def check() -> int:
    errors: list[str] = []
    for p in fragments():
        try:
            parse_fragment(p)
        except FragmentError as e:
            errors.append(str(e))
    for e in errors:
        print(e, file=sys.stderr)
    return 1 if errors else 0


def _unreleased_bounds(text: str) -> tuple[int, int]:
    """Offsets of the body of "## Unreleased": after its heading line, up to the next version heading."""
    m = UNRELEASED.search(text)
    if not m:
        raise SystemExit("CHANGELOG.md has no '## Unreleased' heading")
    start = m.end()
    nxt = VERSION_HEADING.search(text, start)
    end = nxt.start() if nxt else len(text)
    return start, end


def _split_sections(body: str) -> tuple[str, list[tuple[str, str]]]:
    """Split an Unreleased body into its preamble and (heading, content) pairs."""
    heads = list(SECTION_HEADING.finditer(body))
    preamble = body[: heads[0].start()] if heads else body
    sections: list[tuple[str, str]] = []
    for i, h in enumerate(heads):
        content_end = heads[i + 1].start() if i + 1 < len(heads) else len(body)
        sections.append((h.group("name"), body[h.end() : content_end]))
    return preamble, sections


def _already_folded(body: str, section: str, entry: str) -> bool:
    """True if `entry` is a whole item under `section` (a previous render wrote it)."""
    for name, content in _split_sections(body)[1]:
        if name == section and entry in content.strip("\n").split("\n\n"):
            return True
    return False


def fold(text: str, entries: dict[str, list[str]]) -> str:
    """Return CHANGELOG text with entries appended under their sections in "## Unreleased"."""
    start, end = _unreleased_bounds(text)
    body = text[start:end]
    preamble, sections = _split_sections(body)

    for section in SECTIONS:
        items = entries.get(section)
        if not items:
            continue
        block = "\n\n".join(items)
        names = [n for n, _ in sections]
        if section in names:
            i = names.index(section)
            content = sections[i][1].strip("\n")
            sections[i] = (section, (content + "\n\n" if content else "") + block)
        else:
            # Insert after the last existing section that precedes it canonically.
            rank = SECTIONS.index(section)
            at = 0
            for i, n in enumerate(names):
                if n in SECTIONS and SECTIONS.index(n) < rank:
                    at = i + 1
            sections.insert(at, (section, block))

    out = preamble.rstrip("\n") + "\n\n" if preamble.strip() else "\n"
    for name, content in sections:
        out += f"### {name}\n\n" + content.strip("\n") + "\n\n"
    return text[:start] + out + text[end:]


def render(dry_run: bool = False) -> int:
    paths = fragments()
    if not paths:
        print("no fragments under changes/unreleased/")
        return 0
    text = CHANGELOG.read_text(encoding="utf-8")
    start, end = _unreleased_bounds(text)
    entries: dict[str, list[str]] = {}
    for p in paths:
        section, body = parse_fragment(p)
        # Already folded by an interrupted run: only the deletion is outstanding.
        if _already_folded(text[start:end], section, body):
            continue
        entries.setdefault(section, []).append(body)
    new = fold(text, entries) if entries else text
    if dry_run:
        start, end = _unreleased_bounds(new)
        sys.stdout.write("## Unreleased\n" + new[start:end])
        return 0
    CHANGELOG.write_text(new, encoding="utf-8")
    for p in paths:
        p.unlink()
    print(f"folded {len(paths)} fragment(s) into CHANGELOG.md")
    return 0


def release(version: str, date: str | None) -> int:
    rc = render()
    if rc:
        return rc
    version = version.lstrip("v")
    date = date or _dt.date.today().isoformat()
    text = CHANGELOG.read_text(encoding="utf-8")
    m = UNRELEASED.search(text)
    if not m:
        raise SystemExit("CHANGELOG.md has no '## Unreleased' heading")
    heading = f"## Unreleased\n\n## {version} — {date}\n"
    text = text[: m.start()] + heading + text[m.end() :]
    CHANGELOG.write_text(text, encoding="utf-8")
    print(f"CHANGELOG.md: Unreleased is now {version} — {date}")
    return 0


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    sub.add_parser("check", help="validate every fragment")
    r = sub.add_parser("render", help="fold fragments into CHANGELOG.md and delete them")
    r.add_argument("--dry-run", action="store_true", help="print the resulting Unreleased section, change nothing")
    rel = sub.add_parser("release", help="render, then turn Unreleased into a dated version section")
    rel.add_argument("version")
    rel.add_argument("--date", help="YYYY-MM-DD (default: today)")
    a = ap.parse_args(argv)
    if a.cmd == "check":
        return check()
    if a.cmd == "render":
        return render(dry_run=a.dry_run)
    return release(a.version, a.date)


if __name__ == "__main__":
    sys.exit(main())
