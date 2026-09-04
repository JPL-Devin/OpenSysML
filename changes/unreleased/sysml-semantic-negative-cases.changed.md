- **The rejection oracle covers the pilot's SysML validation constraints case by case.** The
  `cmd/pilot-reject/testdata/negative/semantic/` source gains 45 minimal invalid models, one per
  SysML validation constraint of the pinned pilot that had no case before, each header citing the
  constraint by its `validate*` name; seven `grammar/` cases and one `extensions/` case cover the
  requirement, case, state and view body items the pilot rejects as syntax errors outside their
  owning body, and the fourteen existing `xpect/` cases that already covered a constraint now
  name it. The oracle stands at 216 cases, 194 rejected by both implementations and 22 the pilot
  rejects and OpenSysML accepts; `docs/project/pilot-rejection.md` adjudicates every gap, lists
  the constraints the pilot declares but does not enforce and those for which no legal violating
  model exists, and carries a name-by-name census of the 100 SysML constraints. No validation
  rule changes in this entry.
