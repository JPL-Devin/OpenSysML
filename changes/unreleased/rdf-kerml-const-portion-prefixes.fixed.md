- **KerML `const` and `portion` feature prefixes survive a graph-only RDF round trip.**
  The decoder wrote `sysml:isConstant` back as SysML's `constant` under a KerML root, where the
  grammar spells it `const`, so a graph stripped of its `sysx:sourceText` came back unwritable
  (`cannot convert the reference to Prefixes::A from Prefixes::B::k`); it now spells the flag in
  the grammar the root's `sysx:sourceLanguage` records. The encoder did not export
  `Feature::isPortion` at all, so `portion feature p : A;` came back as `composite feature p : A;`
  from the graph alone; it now writes `sysml:isPortion`, which a KerML root reads back as
  `portion` in place of `composite`. On a SysML root the flag is the fact `snapshot`/`timeslice`
  states (the encoder now writes it for those too), and a graph stating it without a
  `sysml:portionKind` is refused rather than respelled, SysML having no `portion` prefix.
