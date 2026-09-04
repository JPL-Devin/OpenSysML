- **Find References and Rename answer in milliseconds on large workspaces.** Each
  `textDocument/references` and `textDocument/rename` request used to re-read every reference
  in every open document and resolve each one afresh, so on a workspace of a hundred files a
  request could take a second or two. The workspace now keeps a reverse reference index —
  every written name, filed under the declaration it denotes and, for an alias, under the alias
  too — rebuilt once on the first such request after an edit and then answered by lookup. The
  results are unchanged: references still list every segment of a qualified name that denotes
  the symbol, alias uses still count for both alias and target, a call tied between overloads
  still names nothing, and rename still edits only the name as written.
