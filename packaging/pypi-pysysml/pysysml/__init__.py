"""Placeholder holding the old PyPI name: raises on import to report the rename.

It deliberately does not re-export `opensysml` — a working alias is a
compatibility shim, and this release exists to end the old name, not extend it.
"""

MESSAGE = """\
pysysml has been renamed to opensysml, and this version of pysysml is a \
placeholder that only reports the rename.

    pip uninstall pysysml
    pip install opensysml

Then `import opensysml` instead of `import pysysml`. The API is the same; what \
changed with it are the names built on the old one:

    PYSYSML_* environment variables  ->  OPENSYSML_*
    ~/.pysysml (state directory)     ->  ~/.opensysml
    PySysMLError                     ->  OpenSysMLError
    pysysml-generate                 ->  opensysml-generate

The service binary is still sysml-grpc, and the wire protocol is unchanged, so \
a new client talks to an old service.

Pinning `pysysml==0.2.0` keeps the last real release working, but it is the end \
of that line: no fix will be published under this name.\
"""

raise ImportError(MESSAGE)
