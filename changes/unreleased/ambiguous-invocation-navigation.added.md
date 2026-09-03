- **Editor navigation on an ambiguous call names every tied overload, and never one of them.**
  A call the checker reports `invocation-ambiguous` used to navigate to whichever declaration
  name resolution found first. Go-to-definition now lists each overload the arguments leave
  tied, hover names them all with their qualified names, find-references on any one of the
  overloads leaves the ambiguous call out, and rename does too — the call is not rewritten to
  a name it was never bound to, and starting a rename from the call itself is refused with
  `the call is ambiguous between several overloads`. A call that selects one overload still
  navigates, lists and renames as that overload.
