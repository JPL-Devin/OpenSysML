# pysysml

Python client for Systemica: parse, inspect and execute SysML v2 models over the
`sysml-grpc` service.

```bash
pip install pysysml             # from PyPI, once the first release is published
pip install -e python/          # or from a checkout, at the repository root
```

```python
import pysysml

model = pysysml.load("model.sysml", strict=True)   # raises on error diagnostics
print(model.eval("1 + 2 * 3"))                     # 7

vehicle = model["Vehicle"]                         # by short name or FQN
inst = model.instantiate("Demo::Vehicle")
inst.mass                                          # 1500.0

model.verify_satisfaction()                        # every assert satisfy … by …
model.save("model.ttl")                            # RDF Turtle
```

Every call goes through the `sysml-grpc` service, which `pysysml` starts for you from
`~/.pysysml/bin/sysml-grpc`; the guide below says how to put it there.

- Using the client:
  [docs/guide/09-python.md](https://github.com/Open-MBEE/Systemica/blob/main/docs/guide/09-python.md)
  — installing the service binary, loading a model, instances, verification, conversion
  and queries
- The API surface, generated typed classes, latency and the module map:
  [docs/reference/python-api.md](https://github.com/Open-MBEE/Systemica/blob/main/docs/reference/python-api.md)
- Installing from source and running the tests:
  [INSTALL.md](https://github.com/Open-MBEE/Systemica/blob/main/python/INSTALL.md)
