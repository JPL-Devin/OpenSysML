- **An unnamed feature takes a name from its reference only in the forms the reference
  validators name.** `perform a;`, `exhibit s;`, `include u;`, `require`/`assume`, `frame`,
  `render`, a variant and a state's `entry`/`do`/`exit` action are members of their owner under
  the referenced feature's name, inside it and through a qualified name or chain from outside
  (`h.a` names the performed `a`, and duplicates the inherited one, as `Duplicate of inherited
  member name` warns in both tools). An `assert q;`, a `satisfy r;`, an `event` and a plain
  `::> q` declare no name, so `h.q` written anywhere still names the inherited `H::q`; `assert
  h.q;` written inside `h` still does not find itself. A member redefining several features is
  named by the first (`part :>> engine :>> motor;` is `engine`), a declared short name suppresses
  the derived name (`part <e> :>> engine;` is `e` alone), and a feature chain names nothing
  (`part :>> p.q;`). Only a redefinition hides the inherited member it names; an ordinary `:>`
  subsetting or a reference no longer masks it, so a feature reached through such a member
  resolves as the reference validators resolve it. A name-conflict warning on a member with a
  derived name is reported on the whole declaration, where the validators place it.
