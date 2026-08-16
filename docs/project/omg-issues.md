# Bugs in the OMG materials

One place to look for defects found in the OMG-published sources this
implementation consumes, outside the training corpus. Corpus files are
adjudicated per file in [training-examples.md](training-examples.md); this page
records defects in the **vendored specification libraries**
(`internal/core/libs/stdlib/`), where the declaration is wrong rather than a
model using it.

Each row quotes the vendored declaration verbatim so a reviewer can judge it
without opening the library, and names what Systemica implements instead. Every
divergence is also a row in [spec-compliance.md](spec-compliance.md).

| Library file | Declaration | What the vendored body says | What we implement | Why |
|---|---|---|---|---|
| `Kernel Libraries/Kernel Function Library/SequenceFunctions.kerml` | `function includingAt` | `(seq->subsequence(1, index - 1), values, seq->subsequence(index + 1))` — the prefix before `index`, then the values, then the tail from `index + 1`, so the element **at** `index` is dropped from the result | insertion: the values are inserted before the 1-based `index`, the tail from that position shifts right, and the result is longer than `seq` by the values inserted. `index == size + 1` appends; any other index outside `1..size + 1` is `ErrIndexOutOfRange` (`runtime.builtinSequenceIncludingAt`) | The body contradicts the declarations around it in the same file. `excludingAt` is the operation that removes at an index, and the behavior pairs are additive/subtractive: `add` calls `including` as `remove` calls `excluding`, and `addAt` calls `includingAt` (`seq->includingAt(values, index)`) as `removeAt` calls `excludingAt`. A removing `includingAt` would leave the library with two ways to delete at an index and none to insert at one, and would make `addAt` remove. The vendored expression is an off-by-one slip in the tail: the insertion body is `(seq->subsequence(1, index - 1), values, seq->subsequence(index))` |

## `includingAt` — the vendored declaration

Quoted verbatim from
`internal/core/libs/stdlib/Kernel Libraries/Kernel Function Library/SequenceFunctions.kerml`:

```kerml
function includingAt{ in seq: Anything[0..*] ordered nonunique; in values: Anything[0..*] ordered nonunique;
    in index: Positive[1];
    return : Anything[0..*] ordered nonunique =
        (seq->subsequence(1, index - 1), values, seq->subsequence(index + 1));
}
```

`subsequence(1, index - 1)` is the prefix ending before `index`, and
`subsequence(index + 1)` is the tail starting after `index`; the element at
`index` appears in neither, so evaluating the body as written *replaces* it with
`values` rather than inserting before it. Systemica implements insertion
(maintainer ruling, PR for task S4), so `includingAt` is a divergence from the
vendored body and is recorded here for review against a future OMG release.
