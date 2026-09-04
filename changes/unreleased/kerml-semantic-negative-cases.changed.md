- **The rejection oracle now names the pilot constraint each case exercises.** A fourth
  corpus source, `cmd/pilot-reject/testdata/negative/semantic/`, adds 43 minimal invalid
  models, one per KerML or shared validation constraint of the pinned pilot that had no case
  before, each header citing the constraint by its `validate*` name. The oracle stands at 163
  cases, 150 rejected by both implementations and 13 the pilot rejects and OpenSysML accepts;
  `docs/project/pilot-rejection.md` adjudicates every gap with the pilot's message and the pass
  that is silent, and lists the constraints the pilot declares but does not enforce and those
  for which no legal violating model exists. No validation rule changes in this entry.
