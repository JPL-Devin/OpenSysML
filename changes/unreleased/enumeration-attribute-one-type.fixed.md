- **Only an `attribute` typed by an enumeration is held to one type.** `An enumeration attribute
  cannot have more than one type` was also raised on a `ref` or bare usage typed by an enumeration
  and another definition, which the reference implementation accepts as a reference usage; and an
  enumerated value whose value is typed by several types is judged by all of them, so `h =
  wrongOrLevel` with `ref wrongOrLevel : Wrong, Level` is reported as typed outside its enumeration.
