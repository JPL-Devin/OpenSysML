- **Both ends of a keyword-first KerML relationship are kind-checked.** `subtype`,
  `subclassifier`, `typing`, `subset`, `redefinition`, `conjugate`, `inverse`, `disjoint` and
  `featuring` members whose source or target resolves to an element the relationship cannot relate
  — a package at any end, a class where only a feature may stand (`subset C :> f`,
  `inverse C of f`, `typing C : T`, `featuring C by T`), a feature where only a classifier may
  (`subclassifier f :> B`) — are now reported at the type tier
  (`<keyword> source|target must be a type|classifier|feature, found <kind>`), as the reference
  implementation reports when it cannot link the typed cross-reference. Previously only the
  declaration clauses (`class C :> Q;`, `feature f : Q;`) were checked and
  `disjoint Q from B;` with `Q` a package analysed clean. Unresolved ends are still left to name
  resolution, and the declaration-clause reports are unchanged, with one exception: a named
  multiplicity is a feature and so a type (KerML 1.0 §8.3.3, Multiplicity specializes Feature),
  so with `multiplicity M [1..2] { feature x; }` the clauses `feature g : M;` and
  `feature g :> M.x;` are no longer reported as `type must be a type, found multiplicity` and
  `feature chain segment must be a feature, found multiplicity` — the reference implementation
  accepts both, as it does the keyword-first spellings.
