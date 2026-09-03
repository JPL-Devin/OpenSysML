"""Count the compliance map's rule rows when the site is built, not in git.

docs/project/spec-compliance.md carries a `<!-- doc-counts:begin census -->` …
`<!-- doc-counts:end census -->` block instead of literal counts, so a pull request
that adds a row never rewrites a shared line. This hook replaces the block with the
census it counts from the page's own rows; `internal/doccounts` counts rows the same way.
"""

import logging
import re

log = logging.getLogger("mkdocs.hooks.census")

PAGE = "project/spec-compliance.md"
BLOCK = re.compile(
    r"^<!-- doc-counts:begin census -->\n.*?^<!-- doc-counts:end census -->\n",
    re.MULTILINE | re.DOTALL,
)
# '⚠' without its variation selector: the map writes both spellings.
MARKERS = ("✅", "⚠", "❌", "⛔", "🚧")
UNREFEREED = "**No external referee:**"


def is_rule_row(line: str) -> bool:
    """A table row carrying exactly one status marker; notes naming several are prose."""
    text = line.strip()
    if not text.startswith("|"):
        return False
    return sum(cell.count(m) for cell in text.strip("|").split("|") for m in MARKERS) == 1


def count_rules(markdown: str) -> dict[str, int]:
    counts = {m: 0 for m in MARKERS}
    counts["self-assessed"] = 0
    unrefereed = False
    for line in markdown.splitlines():
        text = line.strip()
        if text.startswith("#"):
            unrefereed = False
        elif text.startswith(UNREFEREED):
            unrefereed = True
        elif is_rule_row(text):
            counts[next(m for m in MARKERS if m in text)] += 1
            if unrefereed:
                counts["self-assessed"] += 1
    counts["total"] = sum(counts[m] for m in MARKERS)
    return counts


def census(markdown: str) -> str:
    """Return the page with its census block replaced by the counted sentence."""
    if not BLOCK.search(markdown):
        raise ValueError(f"{PAGE}: no doc-counts census block")
    c = count_rules(markdown)
    if c["total"] == 0:
        raise ValueError(f"{PAGE}: no rule rows to count")
    if c["🚧"]:
        raise ValueError(f"{PAGE}: {c['🚧']} 🚧 rows; give them a status the census states")
    sentence = (
        f"The map below tracks {c['total']} semantic rules: **{c['✅']} ✅ faithful, "
        f"{c['⚠']} ⚠️ approximate, {c['❌']} ❌ not implemented, {c['⛔']} ⛔ deliberate divergence**; "
        f"{c['self-assessed']} of them have no external referee.\n"
    )
    return BLOCK.sub(lambda _: sentence, markdown, count=1)


def on_page_markdown(markdown: str, page, **_kwargs) -> str:
    if page.file.src_uri != PAGE:
        return markdown
    try:
        return census(markdown)
    except ValueError as e:
        log.warning("%s", e)
        return markdown
