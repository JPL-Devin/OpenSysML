#!/usr/bin/env python3
"""Gate a pysysml release on the tag and the declared version agreeing.

Used by the `publish-pypi` CircleCI job before anything is built or uploaded:
a PyPI version can be yanked but never re-uploaded, so a tag that does not name
the version the package would publish must fail here rather than after the fact.

    python scripts/check_version.py --tag pysysml-v0.1.0

Prints the version the tag names on success. With `--pre-release` it prints
`yes`/`no` instead, which the job uses to route a pre-release tag to TestPyPI.
"""

import argparse
import ast
import os
import sys

from packaging.version import InvalidVersion, Version

TAG_PREFIX = "pysysml-v"

VERSION_FILE = os.path.join(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "pysysml", "_version.py"
)


class VersionError(Exception):
    """A tag that does not name a publishable pysysml version."""


def declared_version(version_file=VERSION_FILE):
    """The version declared in pysysml/_version.py.

    Read rather than imported, so the check needs neither the package's
    dependencies nor an installed distribution.

    Args:
        version_file (str): Path to pysysml/_version.py

    Returns:
        str: The declared version

    Raises:
        VersionError: If the file declares no VERSION string
    """
    with open(version_file, encoding="utf-8") as f:
        module = ast.parse(f.read(), filename=version_file)
    for node in module.body:
        if isinstance(node, ast.Assign) and any(
            isinstance(t, ast.Name) and t.id == "VERSION" for t in node.targets
        ):
            if isinstance(node.value, ast.Constant) and isinstance(node.value.value, str):
                return node.value.value
    raise VersionError(f"{version_file} declares no VERSION string")


def version_from_tag(tag, version=None):
    """The version a release tag names, checked against the declared version.

    Args:
        tag (str): Release tag, e.g. 'pysysml-v0.1.0'
        version (str, optional): Declared version; read from
            pysysml/_version.py when omitted

    Returns:
        str: The version to publish

    Raises:
        VersionError: If the tag is empty, is not a pysysml tag, or names a
            version other than the declared one
    """
    declared = version if version is not None else declared_version()
    if not tag:
        raise VersionError(
            "No tag given. The publish job runs on a "
            f"{TAG_PREFIX}<version> tag and reads CIRCLE_TAG."
        )
    if not tag.startswith(TAG_PREFIX):
        raise VersionError(
            f"Tag {tag!r} does not start with {TAG_PREFIX!r}. A pysysml release "
            f"is cut by a {TAG_PREFIX}<version> tag; a core release tag (v*) "
            "publishes the binaries and the GitHub release, not the package."
        )
    tag_version = tag[len(TAG_PREFIX):]
    if tag_version != declared:
        raise VersionError(
            f"Tag {tag!r} names version {tag_version!r}, but "
            f"python/pysysml/_version.py declares {declared!r}. "
            "Publishing would put a version on PyPI that the package does not "
            "report. Fix one of the two and tag again."
        )
    return tag_version


def is_pre_release(version):
    """Whether a version is a PEP 440 pre-release (alpha/beta/rc).

    Args:
        version (str): Version string

    Returns:
        bool: True for a pre-release, which is published to TestPyPI

    Raises:
        VersionError: If the version is not a valid PEP 440 version
    """
    try:
        return Version(version).is_prerelease
    except InvalidVersion as e:
        raise VersionError(f"{version!r} is not a PEP 440 version: {e}")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--tag",
        default=os.environ.get("CIRCLE_TAG", ""),
        help="release tag (default: $CIRCLE_TAG)",
    )
    parser.add_argument(
        "--pre-release",
        action="store_true",
        help="print yes/no for whether the tag names a pre-release",
    )
    args = parser.parse_args(argv)

    try:
        version = version_from_tag(args.tag)
        pre_release = is_pre_release(version)
    except VersionError as e:
        print(f"error: {e}", file=sys.stderr)
        return 1

    if args.pre_release:
        print("yes" if pre_release else "no")
    else:
        print(version)
    return 0


if __name__ == "__main__":
    sys.exit(main())
