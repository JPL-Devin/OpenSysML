- **The LSP refuses a rename that would collide, capture or shadow.** `textDocument/rename`
  checked only that the new name was spelled like an identifier, then rewrote the declaration
  and its references: renaming `x` to `y` where the owner already declares `y` produced two
  members of one name, and renaming to a name some enclosing, imported or intervening scope
  declares silently rebound every rewritten reference — with no diagnostic, since the name still
  resolves. The rename now runs the batch edit API's check, moved to a package both share
  (`internal/core/rename`): it is refused, with an error the editor shows naming the element the
  new name would mean, when the new long or short name already means something where the element
  is declared (a sibling's long or short name included), or when any reference it rewrites — in
  any open workspace document, as a whole name, a segment of a qualified one or a feature chain's
  member — would read another element afterwards. Each rewritten segment is checked by a trial
  reading of its reference with the new spelling, so a chain member is read in its operand's type
  rather than where the chain is written; the batch edit API gains that check too, where it
  previously let `d.x` be renamed onto a `y` that `d`'s type declares. A name taken only in an
  unrelated scope is not a conflict, nor is renaming a name to itself, and a shorthand
  redefinition whose declaration and reference share one span still renames. Aliases, short-name
  references and out-of-workspace declarations keep their rules, and
  `textDocument/prepareRename` is unchanged.
