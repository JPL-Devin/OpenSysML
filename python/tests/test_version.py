"""Tests that the pysysml version has exactly one source of truth.

The version used to be written in three places (`pyproject.toml`, `setup.py`
and `pysysml/__init__.py`) and drifted from the repository's own line. It is now
declared once, in `pysysml/_version.py`: the packaging metadata reads it and
`pysysml.__version__` reports the installed distribution's version. These tests
fail if a second declaration reappears or the two stop agreeing.
"""

import importlib.util
import os
import re
import subprocess
import sys
from importlib.metadata import PackageNotFoundError, version as distribution_version

import pytest

import pysysml
from pysysml._version import VERSION

PYTHON_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CHECK_VERSION = os.path.join(PYTHON_DIR, "scripts", "check_version.py")


def _load_check_version():
    """Load scripts/check_version.py, which is release tooling, not a module."""
    spec = importlib.util.spec_from_file_location("check_version", CHECK_VERSION)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


check_version = _load_check_version()


def test_declared_version_is_pep440():
    """The declared version is a version PyPI will accept."""
    from packaging.version import Version

    assert str(Version(VERSION)) == VERSION


def test_dunder_version_matches_declaration():
    """`pysysml.__version__` agrees with the single declaration.

    Installed, it comes from the distribution metadata, which setuptools filled
    in from `_version.py`; from a source tree it falls back to the declaration
    itself. Either way the two must not disagree.
    """
    assert pysysml.__version__ == VERSION


def test_installed_metadata_matches_declaration():
    """The installed distribution's version comes from the declaration."""
    try:
        installed = distribution_version("pysysml")
    except PackageNotFoundError:
        pytest.skip("pysysml is not installed; nothing to compare metadata against")
    assert installed == VERSION


def test_no_second_version_declaration():
    """No file under python/ hard-codes a version literal of its own.

    `_version.py` holds the declaration and the tests compare against it; any
    other `version = "x.y.z"` is the duplication this collapsed.
    """
    pattern = re.compile(r"""^\s*(?:__version__|version)\s*=\s*["']\d+\.\d+""")
    offenders = []
    for root, dirs, files in os.walk(PYTHON_DIR):
        dirs[:] = [
            d for d in dirs
            if d not in {".git", "build", "dist", "__pycache__", ".venv"}
            and not d.endswith(".egg-info")
        ]
        for name in files:
            if not name.endswith((".py", ".toml", ".cfg")):
                continue
            path = os.path.join(root, name)
            if os.path.samefile(path, os.path.join(PYTHON_DIR, "pysysml", "_version.py")):
                continue
            with open(path, encoding="utf-8") as f:
                for lineno, line in enumerate(f, 1):
                    if pattern.match(line):
                        offenders.append(f"{os.path.relpath(path, PYTHON_DIR)}:{lineno}")
    assert offenders == [], (
        "version literals outside pysysml/_version.py: " + ", ".join(offenders)
    )


def test_setup_py_is_gone():
    """pyproject.toml declares the build; a setup.py would be a second answer."""
    assert not os.path.exists(os.path.join(PYTHON_DIR, "setup.py"))


def test_version_from_tag_accepts_the_matching_tag():
    """A tag naming the declared version yields that version."""
    assert check_version.version_from_tag(f"pysysml-v{VERSION}") == VERSION


@pytest.mark.parametrize("tag, expected", [
    ("", "reads CIRCLE_TAG"),
    ("v0.0.5", "does not start with"),
    ("pysysml-0.1.0", "does not start with"),
    ("pysysml-v9.9.9", "declares"),
])
def test_version_from_tag_rejects(tag, expected):
    """A tag that would publish the wrong version fails, with a reason."""
    with pytest.raises(check_version.VersionError, match=expected):
        check_version.version_from_tag(tag)


@pytest.mark.parametrize("version, pre", [
    ("0.1.0", False),
    ("1.2.3", False),
    ("0.1.0rc1", True),
    ("0.1.0a1", True),
    ("0.1.0b2", True),
])
def test_pre_release_detection(version, pre):
    """Pre-release versions route to TestPyPI; final ones to PyPI."""
    assert check_version.is_pre_release(version) is pre


def test_check_version_cli_reports_the_version():
    """The job reads the version to publish off this script's stdout."""
    out = subprocess.run(
        [sys.executable, CHECK_VERSION, "--tag", f"pysysml-v{VERSION}"],
        capture_output=True, text=True, check=True,
    )
    assert out.stdout.strip() == VERSION


def test_check_version_cli_fails_on_a_mismatched_tag():
    """A mismatch fails the job before anything is built or uploaded."""
    out = subprocess.run(
        [sys.executable, CHECK_VERSION, "--tag", "pysysml-v9.9.9"],
        capture_output=True, text=True,
    )
    assert out.returncode == 1
    assert "9.9.9" in out.stderr and VERSION in out.stderr
