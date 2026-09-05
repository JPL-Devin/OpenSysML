- **A parser benchmark over a real model, and its Apollo 11 figures on the landing page.**
  `BenchmarkParseModel` in `internal/core/parser` parses every `.sysml` and `.kerml` file under
  the directory `OPENSYSML_BENCH_MODEL` names, with no library, resolution or validation, so the
  parser's own cost is measurable apart from a load's. Its figures for the public Apollo 11 model
  (8 ms to parse, 0.37 s to validate) close the landing page and open the README, and
  `docs/internals/performance.md` records the measurement, the commands to repeat it, and what
  the run reports about the model.
