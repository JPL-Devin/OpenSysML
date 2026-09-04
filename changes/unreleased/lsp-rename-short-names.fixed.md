- **The LSP renames one of an element's names at a time.** Renaming the long name of
  `part def <O> Old;` rewrote every reference that resolved to it, `part a : O;` included, although
  a reference spelled with the short name still resolves after the rename; the batch edit API
  already left such references alone. `textDocument/rename` now rewrites only the references
  spelled with the name under the cursor, as one whole name or as a segment of a qualified one
  (`P::Old::x` changes, `P::O::x` does not), and renames a short name in its own right: with the
  cursor on `<O>` or on a reference written `O`, `textDocument/prepareRename` offers the short
  name's span and the rename rewrites it and every reference written `O`, leaving the `Old` ones.
  Both hold for definitions, usages, packages and aliases, across the workspace's documents, and
  compose with aliases: renaming `Old` leaves the `O` in `alias X for P::O;` alone.
