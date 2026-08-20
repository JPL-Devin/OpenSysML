# opensysml has been renamed back to pysysml

This distribution is a placeholder. It holds the `opensysml` name on PyPI so that
installing it reports the rename rather than silently resolving to 0.3.0, the
last release under this name that contains the client.

```bash
pip uninstall opensysml
pip install pysysml
```

Then import the current name:

```python
import pysysml

with pysysml.connect() as client:
    model = client.load("model.sysml")
```

Importing `opensysml` from this version raises `ImportError` with the same
instructions. It does not re-export `pysysml`: a working alias would be a
compatibility shim, and the project renamed rather than aliased.

## What changed with the name

| Old | New |
| --- | --- |
| `pip install opensysml`, `import opensysml` | `pip install pysysml`, `import pysysml` |
| `OPENSYSML_*` environment variables | `PYSYSML_*` |
| `~/.opensysml` state directory | `~/.pysysml` |
| `OpenSysMLError` | `PySysMLError` |
| `opensysml-generate` | `pysysml-generate` |
| `opensysml-v*` release tag | `pysysml-v*` |

The API is otherwise unchanged, as is the `sysml-grpc` service binary and its
wire protocol, so a new client talks to an already-installed service. A
`~/.opensysml` left behind by an old install is never read again and can be
deleted.

Pinning `opensysml==0.3.0` keeps the last real release working, but nothing
further will be published under this name.

- Project: https://github.com/Open-MBEE/OpenSysML
- Package: https://pypi.org/project/pysysml/
