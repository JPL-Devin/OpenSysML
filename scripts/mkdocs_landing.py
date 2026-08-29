"""Render docs/README.md with the landing page template, and check the templates' links.

Selecting the template here rather than in front matter keeps the page plain Markdown
on GitHub; the site-relative links every template in overrides/ holds, which MkDocs
cannot validate, are checked against the built file set so a renamed page fails
`--strict` like any link.
"""

import logging
import re
from pathlib import Path

log = logging.getLogger("mkdocs.hooks.landing")

LANDING = "README.md"
TEMPLATE = "home.html"

# `{{ 'guide/01-install/'|url }}` — the site-relative targets the template links to.
URL_FILTER = re.compile(r"""\{\{\s*['"]([^'"]+)['"]\s*\|\s*url\s*\}\}""")


def _source_candidates(target: str) -> list[str]:
    """The docs/ paths that would publish at `target`, under use_directory_urls."""
    path = target.strip("/")
    if not path:
        return ["index.md", LANDING]
    return [f"{path}.md", f"{path}/index.md", f"{path}/{LANDING}"]


def on_files(files, config):
    overrides = Path(config["theme"].custom_dir or "")
    if not (overrides / TEMPLATE).is_file():
        log.warning("the landing page template %s is missing", overrides / TEMPLATE)
        return files

    present = {file.src_uri for file in files}
    for template in sorted(overrides.glob("**/*.html")):
        targets = dict.fromkeys(URL_FILTER.findall(template.read_text()))
        for target in targets:
            if not any(candidate in present for candidate in _source_candidates(target)):
                log.warning(
                    "%s links to %r, which no page publishes",
                    template.relative_to(overrides),
                    target,
                )
    return files


def on_page_markdown(markdown: str, page, **_kwargs) -> str:
    if page.file.src_uri == LANDING:
        # The hero wants the full width, and the map below it is its own contents list.
        page.meta["template"] = TEMPLATE
        page.meta["hide"] = ["navigation", "toc"]
    return markdown
