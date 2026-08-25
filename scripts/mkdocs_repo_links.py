"""Rewrite links to unpublished or out-of-docs files to the repository.

A page's `../../examples/foo.sysml` is a real file on GitHub but has no page on the
site, so it is rewritten to the repository here instead of being hard-coded in the
page. A link to a docs/ page excluded from the published site follows the same rule.
Links to published pages inside docs/ are left as written, so MkDocs still validates
them.
"""

import logging
import re
from pathlib import Path

log = logging.getLogger("mkdocs.hooks.repo_links")

# Inline links and images; docs/ uses no reference-style links.
LINK = re.compile(r"(!?\[[^\]]*\]\()([^)\s]+)((?:\s+\"[^\"]*\")?\))")
FENCE = re.compile(r"^ {0,3}(`{3,}|~{3,})(.*)$")
SCHEME = re.compile(r"^[a-zA-Z][a-zA-Z0-9+.-]*:")


def _rewrite(
    target: str,
    page_dir: Path,
    docs_dir: Path,
    repo_root: Path,
    repo_url: str,
    files,
) -> str:
    if not target or target.startswith(("#", "/")) or SCHEME.match(target):
        return target

    path, _, fragment = target.partition("#")
    if not path:
        return target

    resolved = (page_dir / path).resolve()
    if resolved == docs_dir or docs_dir in resolved.parents:
        # A directory is a listing on GitHub; here it means the section's index.
        relative = resolved.relative_to(docs_dir).as_posix()
        if resolved.is_dir() and (resolved / "README.md").is_file():
            readme = f"{relative.rstrip('/')}/README.md".lstrip("/")
            readme_file = files.get_file_from_path(readme)
            if readme_file and readme_file.inclusion.is_included():
                local = f"{path.rstrip('/')}/README.md"
                return local + (f"#{fragment}" if fragment else "")
        file = files.get_file_from_path(relative)
        if file and file.inclusion.is_included():
            return target

    try:
        relative = resolved.relative_to(repo_root)
    except ValueError:
        log.warning("link %r in %s escapes the repository", target, page_dir)
        return target

    kind = "tree" if resolved.is_dir() else "blob"
    url = f"{repo_url}/{kind}/main/{relative.as_posix()}"
    return f"{url}#{fragment}" if fragment else url


def on_page_markdown(markdown: str, page, config, files) -> str:
    repo_url = (config.get("repo_url") or "").rstrip("/")
    if not repo_url:
        return markdown

    repo_root = Path(config["config_file_path"]).resolve().parent
    docs_dir = Path(config["docs_dir"]).resolve()
    page_dir = (docs_dir / page.file.src_uri).resolve().parent

    def substitute(match: re.Match) -> str:
        target = _rewrite(match.group(2), page_dir, docs_dir, repo_root, repo_url, files)
        return f"{match.group(1)}{target}{match.group(3)}"

    out, fence = [], ""
    for line in markdown.splitlines(keepends=True):
        marker = FENCE.match(line)
        if marker:
            run, info = marker.group(1), marker.group(2).strip()
            if not fence:
                fence = run
                out.append(line)
                continue
            # CommonMark: a closing fence is the same character, at least as long,
            # and carries no info string — so ```markdown showing ```bash stays open.
            if not info and run[0] == fence[0] and len(run) >= len(fence):
                fence = ""
                out.append(line)
                continue
        out.append(line if fence else LINK.sub(substitute, line))
    return "".join(out)
