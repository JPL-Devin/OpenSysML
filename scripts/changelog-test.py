#!/usr/bin/env python3
"""Tests for scripts/changelog.py. Run: python3 scripts/changelog-test.py"""

from __future__ import annotations

import importlib.util
import pathlib
import sys
import tempfile
import unittest

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("changelog", HERE / "changelog.py")
changelog = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(changelog)

BASE = """# Changelog

Intro.

## Unreleased

### Added

- **Old added entry.** Text.

### Fixed

- **Old fixed entry.** Text.

## 0.4.3 — 2026-09-01

### Added

- Released thing.
"""


class FoldTest(unittest.TestCase):
    def test_appends_to_existing_section_and_creates_missing_in_order(self):
        out = changelog.fold(
            BASE,
            {
                "Added": ["- **New added.** A."],
                "Changed": ["- **New changed.** C."],
                "Performance": ["- **Faster.** P."],
            },
        )
        unreleased = out[out.index("## Unreleased") : out.index("## 0.4.3")]
        order = [l for l in unreleased.splitlines() if l.startswith("### ")]
        self.assertEqual(order, ["### Added", "### Changed", "### Fixed", "### Performance"])
        self.assertIn("- **Old added entry.** Text.\n\n- **New added.** A.\n\n### Changed", unreleased)
        self.assertIn("### Changed\n\n- **New changed.** C.\n\n### Fixed", unreleased)
        self.assertTrue(unreleased.endswith("- **Faster.** P.\n\n"))
        # Released sections are untouched.
        self.assertTrue(out.endswith("## 0.4.3 — 2026-09-01\n\n### Added\n\n- Released thing.\n"))

    def test_empty_unreleased(self):
        text = "# Changelog\n\n## Unreleased\n\n## 0.1.0 — 2026-01-01\n\n- x.\n"
        out = changelog.fold(text, {"Fixed": ["- **F.** f."]})
        self.assertEqual(
            out, "# Changelog\n\n## Unreleased\n\n### Fixed\n\n- **F.** f.\n\n## 0.1.0 — 2026-01-01\n\n- x.\n"
        )

    def test_unreleased_last_in_file(self):
        text = "# Changelog\n\n## Unreleased\n"
        out = changelog.fold(text, {"Added": ["- a."]})
        self.assertEqual(out, "# Changelog\n\n## Unreleased\n\n### Added\n\n- a.\n\n")

    def test_new_section_lands_after_its_canonical_predecessors(self):
        # The real file's Unreleased section is not in canonical order; existing
        # sections stay put and a new one goes after the last that precedes it.
        text = "## Unreleased\n\n### Added\n\n- a.\n\n### Performance\n\n- p.\n\n### Fixed\n\n- f.\n\n### Changed\n\n- c.\n"
        out = changelog.fold(text, {"Security": ["- s."], "Deprecated": ["- d."]})
        order = [l for l in out.splitlines() if l.startswith("### ")]
        self.assertEqual(
            order, ["### Added", "### Performance", "### Fixed", "### Security", "### Changed", "### Deprecated"]
        )


class RenderTest(unittest.TestCase):
    def setUp(self):
        d = pathlib.Path(tempfile.mkdtemp())
        self.saved = (changelog.CHANGELOG, changelog.FRAGMENTS)
        changelog.CHANGELOG = d / "CHANGELOG.md"
        changelog.FRAGMENTS = d / "changes" / "unreleased"
        changelog.FRAGMENTS.mkdir(parents=True)
        changelog.CHANGELOG.write_text(BASE, encoding="utf-8")

    def tearDown(self):
        changelog.CHANGELOG, changelog.FRAGMENTS = self.saved

    def test_rerun_after_interrupted_render_does_not_duplicate(self):
        (changelog.FRAGMENTS / "a.added.md").write_text("- **A.** a.\n", encoding="utf-8")
        (changelog.FRAGMENTS / "b.fixed.md").write_text("- **B.** b.\n", encoding="utf-8")
        self.assertEqual(changelog.render(), 0)
        once = changelog.CHANGELOG.read_text(encoding="utf-8")
        self.assertEqual(sorted(p.name for p in changelog.FRAGMENTS.iterdir()), [])
        # Simulate a run that wrote the changelog but died before deleting a fragment.
        (changelog.FRAGMENTS / "b.fixed.md").write_text("- **B.** b.\n", encoding="utf-8")
        self.assertEqual(changelog.render(), 0)
        self.assertEqual(changelog.CHANGELOG.read_text(encoding="utf-8"), once)
        self.assertEqual(sorted(p.name for p in changelog.FRAGMENTS.iterdir()), [])


class FragmentTest(unittest.TestCase):
    def _frag(self, name: str, body: str) -> pathlib.Path:
        d = pathlib.Path(tempfile.mkdtemp())
        p = d / name
        p.write_text(body, encoding="utf-8")
        return p

    def test_valid(self):
        p = self._frag("repl-send.added.md", "- **X.** y.\n  more.\n")
        self.assertEqual(changelog.parse_fragment(p), ("Added", "- **X.** y.\n  more."))

    def test_bad_section(self):
        p = self._frag("repl-send.new.md", "- x")
        with self.assertRaises(changelog.FragmentError):
            changelog.parse_fragment(p)

    def test_bad_name(self):
        p = self._frag("repl-send.md", "- x")
        with self.assertRaises(changelog.FragmentError):
            changelog.parse_fragment(p)

    def test_empty(self):
        p = self._frag("a.fixed.md", "\n\n")
        with self.assertRaises(changelog.FragmentError):
            changelog.parse_fragment(p)

    def test_heading_rejected(self):
        p = self._frag("a.fixed.md", "### Fixed\n\n- x")
        with self.assertRaises(changelog.FragmentError):
            changelog.parse_fragment(p)

    def test_not_a_list_item(self):
        p = self._frag("a.fixed.md", "Plain prose.")
        with self.assertRaises(changelog.FragmentError):
            changelog.parse_fragment(p)


if __name__ == "__main__":
    sys.exit(unittest.main())
