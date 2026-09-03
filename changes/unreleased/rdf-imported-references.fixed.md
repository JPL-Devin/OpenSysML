- **A reference reached through an import or an alias links to its element in RDF.**
  `sysml -convert ttl` wrote `sysml:type "BudgetLedger"` for `item b2 : BudgetLedger;` under a
  `public import OtherPkg::*;`, and `sysml:referent "Tempo::operative"` for the value of
  `attribute t : Tempo = Tempo::operative;` — string literals, though the fully qualified
  `OtherPkg::BudgetLedger` a line above linked to `elmt:OtherPkg__BudgetLedger`. The encoder
  looked a name up only along the owner's own namespaces; it now asks name resolution, so a type,
  subsetting, redefinition, relationship end or feature reference spelled through an import, an
  alias or a nested package path links to the same element its fully qualified spelling does.
  A feature chain's member (`w.size`) and a behavior's endpoints (`then idle`) link the same
  way, a chain's member found in its operand's type. A name that genuinely does not resolve is
  still carried as a literal; read back, a linked redefinition target is written by the
  redefined feature's own name where one feature of that name is inherited, qualified where
  several are (Open-MBEE/OpenSysML#90).
