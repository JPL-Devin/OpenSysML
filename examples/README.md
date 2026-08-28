# Examples

This directory contains example SysML v2 models.

## OMG Training Examples

The official OMG SysML v2 training examples are **not included in this repository**.

**Download them with:**

```bash
./scripts/download-training-examples.sh
```

That fetches `sysml/src/training` from the pinned pilot release
(https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation) into
`sysml-v2-training/`, which is gitignored.

**Status:** the corpus gate's current result is in
[docs/project/training-examples.md](../docs/project/training-examples.md), with the files that
still report errors and why.

## Walkthroughs

Each of these is a model and a walkthrough of the commands that exercise it.

| Model | Walkthrough | What it demonstrates |
| --- | --- | --- |
| [rover-demo/rover.sysml](rover-demo/rover.sysml) | [rover-demo/README.md](rover-demo/README.md) | one rover, end to end: structure, calculations, an action with a fork/join and a branch, a hierarchical state machine an object exhibits, the solver commands, the view renderings, and [the same questions from Python](rover-demo/rover_demo.py) |
| [solver-demo.sysml](solver-demo.sysml) | [SOLVER-DEMO.md](SOLVER-DEMO.md) | `%check`, `%explain`, `%solve`, `%configure` and `%optimize` — what conditions *can* hold, which conflict, what satisfies them, which variants are permitted, what is best (needs z3 or cvc5) |
| [views-demo.sysml](views-demo.sysml) | [VIEWS-DEMO.md](VIEWS-DEMO.md) | `%view` and `%render` — the five rendering kinds, the text/Mermaid/Markdown forms, viewpoint conformance and filtered exposure |
| [action-executor-demo.sysml](action-executor-demo.sysml) | [ACTION-EXECUTOR-DEMO.md](ACTION-EXECUTOR-DEMO.md) | executing actions, and stepping one in the REPL |
| `parser_features_demo_*.sysml`/`.kerml` | [PARSER_FEATURES_DEMOS.md](PARSER_FEATURES_DEMOS.md) | the notation the parser accepts, feature by feature |

## Other Examples

Add your own example models here! SysML v2 files use `.sysml` extension, KerML files use `.kerml`.
