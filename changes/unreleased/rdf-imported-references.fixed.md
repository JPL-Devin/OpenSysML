- **A reference reached through an import or an alias links to its element in RDF.**
  `sysml -convert ttl` wrote `sysml:type "BudgetLedger"` for `item b2 : BudgetLedger;` under a
  `public import OtherPkg::*;`, and `sysml:referent "Tempo::operative"` for the value of
  `attribute t : Tempo = Tempo::operative;` — string literals, though the fully qualified
  `OtherPkg::BudgetLedger` a line above linked to `elmt:OtherPkg__BudgetLedger`. The encoder
  looked a name up only along the owner's own namespaces; it now asks name resolution, so a type,
  subsetting, redefinition, relationship end or feature reference spelled through an import, an
  alias or a nested package path links to the same element its fully qualified spelling does.
  A name that genuinely does not resolve is still carried as a literal, and a redefinition read
  back from a linked target is written by the redefined feature's own name
  (Open-MBEE/OpenSysML#90).
