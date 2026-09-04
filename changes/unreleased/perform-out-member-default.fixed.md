- A performed action whose binding declares an `out` parameter with a value
  (`perform action tick { out total : Integer = 7; first start; then done; }`) starts and
  answers that value, rather than failing the performer with `output action parameter given
  as input`: the value of an `out` member is the answer's default, not an argument, so only
  `in`/`inout` members are bound as inputs.
