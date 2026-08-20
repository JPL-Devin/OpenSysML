"""The one place the pysysml version is written.

`pyproject.toml` reads `VERSION` from here as the distribution's version, and
`pysysml.__version__` reports the version of the *installed* distribution, so
neither is a second copy that can drift. `scripts/check_version.py` compares
this value with the release tag before anything is uploaded.
"""

VERSION = "0.4.0"
