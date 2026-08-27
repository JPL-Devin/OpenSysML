"""Check that the header override is the theme's shipped header plus the menu include.

The build reconstructs it, so a theme upgrade that rewrites the header warns (and under
`--strict` fails) rather than silently publishing the previous version's header.
"""

import logging
from pathlib import Path

log = logging.getLogger("mkdocs.hooks.header_menu")

PARTIAL = Path("partials/header.html")
NOTE = """{#-
  The theme's shipped header plus the menu include; re-copy it rather than editing here.
  scripts/mkdocs_header_menu.py checks that on every build.
-#}
"""
ANCHOR = "    {% if config.repo_url %}"
INCLUDE = '    {% include "partials/menu.html" %}\n'


def _shipped(config) -> Path | None:
    """The header partial as the theme ships it, ignoring the override."""
    custom_dir = config["theme"].custom_dir
    for directory in config["theme"].dirs:
        if custom_dir and Path(directory) == Path(custom_dir):
            continue
        candidate = Path(directory) / PARTIAL
        if candidate.is_file():
            return candidate
    return None


def on_config(config):
    custom_dir = config["theme"].custom_dir
    override = Path(custom_dir or "") / PARTIAL
    if not override.is_file():
        log.warning("the header override %s is missing", override)
        return config

    shipped = _shipped(config)
    if shipped is None:
        log.warning("the theme ships no %s to check the override against", PARTIAL)
        return config

    body = shipped.read_text()
    if body.count(ANCHOR) != 1:
        log.warning(
            "%s no longer holds exactly one %r, so the menu's place in the header is "
            "no longer known; re-derive %s",
            shipped,
            ANCHOR,
            override,
        )
        return config

    expected = NOTE + body.replace(ANCHOR, INCLUDE + ANCHOR)
    if override.read_text() != expected:
        log.warning(
            "%s is not %s with the menu include inserted; the theme's header changed, "
            "so re-copy it and insert %r again",
            override,
            shipped,
            INCLUDE.strip(),
        )
    return config
