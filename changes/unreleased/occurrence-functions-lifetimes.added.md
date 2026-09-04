- **`OccurrenceFunctions` evaluate: `'==='`, `isDuring`, `create`, `destroy`, `addNew` and
  `addNewAt` answer over the runtime's occurrences.** Every object the runtime materializes and
  every action or state performance it runs now has a lifetime, kept in a side table in the
  runtime's own execution order (never wall-clock time), so no library frame member is added
  to an object and `%features` keeps its shape. `a === b` and `OccurrenceFunctions::'==='(a, b)`
  agree: `true` only for one and the same occurrence, so two structurally equal parts are
  `!==` while their attributes are `==`. `isDuring(occ)` is `true` while `occ` is alive at the
  evaluation — an object until it is destroyed, a performed action or exhibited state until it
  completes. `create(occ)` begins an occurrence the call is the first to reach; `destroy(occ)`
  ends it with the parts it owns, after which `isDuring` is `false`, any feature read or write
  is `occurrence was destroyed` rather than a stale value, `%features` prints the destruction
  and `%instances` marks the object `destroyed`; `addNew`/`addNewAt` create and insert into an
  ordered group, an index outside `1..size + 1` being `index out of range`. A data value, an
  empty or several-valued argument, a second `destroy`, or an object whose behavior is still
  performing is a typed error naming the function and the parameter. The execution trace
  records `create:` and `destroy:` events.
- **Known limitation:** `addNew` and `addNewAt` answer the group after insertion rather than
  the declared `occ`, since an expression call cannot write its `inout group` argument back;
  write the result with `assign spares := addNew(spares, spare)`.
