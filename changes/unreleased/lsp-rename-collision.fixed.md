- **The LSP refuses a rename that would collide, capture or shadow.** `textDocument/rename`
  checked only that the new name was spelled like an identifier, then rewrote the declaration
  and its references: renaming `x` to `y` where the owner already declares `y` produced two
  members of one name, and renaming to a name some enclosing, imported or intervening scope
  declares silently rebound every rewritten reference — with no diagnostic, since the name still
  resolves. The rename now runs the batch edit API's check, moved to a package both share
  (`internal/core/rename`): it is refused, with an error the editor shows naming the element the
  new name would mean, when the new long or short name already means something where the element
  is declared (a sibling's long or short name included), or when any reference it rewrites — in
  any open workspace document, as a whole name or as a segment of a qualified one — would read
  another element afterwards. A name taken only in an unrelated scope is not a conflict, and a
  shorthand redefinition whose declaration and reference share one span still renames. Aliases,
  short-name references and out-of-workspace declarations keep their rules, and
  `textDocument/prepareRename` is unchanged.
