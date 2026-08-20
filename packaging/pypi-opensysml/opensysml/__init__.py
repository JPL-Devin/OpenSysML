"""Placeholder holding the `opensysml` PyPI name: raises on import to report the rename.

It deliberately does not re-export `pysysml` — a working alias is a
compatibility shim, and this release exists to end this name, not extend it.
"""

MESSAGE = """\
opensysml has been renamed back to pysysml, and this version of opensysml is a \
placeholder that only reports the rename.

    pip uninstall opensysml
    pip install pysysml

Then `import pysysml` instead of `import opensysml`. The API is the same; what \
changed with it are the names built on the old one:

    OPENSYSML_* environment variables  ->  PYSYSML_*
    ~/.opensysml (state directory)     ->  ~/.pysysml
    OpenSysMLError                     ->  PySysMLError
    opensysml-generate                 ->  pysysml-generate

The service binary is still sysml-grpc, and the wire protocol is unchanged, so \
a new client talks to an old service.

Pinning `opensysml==0.3.0` keeps the last real release working, but it is the \
end of that line: no fix will be published under this name.\
"""

raise ImportError(MESSAGE)
