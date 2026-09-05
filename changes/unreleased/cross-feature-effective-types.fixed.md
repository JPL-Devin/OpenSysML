- **A cross feature is compared with its end by effective type, not by spelling.** `end b : A
  crosses a.x` with `feature x;` untyped, `end b crosses a.x` with an untyped end, and an end
  whose body declares `member feature ac : B` while the end itself is typed by `A` used to pass
  `Cross feature must have same type as feature`; they are now reported, as the pilot
  implementation reports them, because an untyped feature is typed by `Anything` and an owned
  cross feature is typed by its end. Each side's types are the KerML `Feature::type` set — the
  declared types of the feature and of every feature it subsets, redefines or references, less
  those a more specific one makes redundant — so `feature x : A subsets w` with `w : W` is
  reported against an end typed `A`, while a connector end that reference-subsets a feature is
  not reported for it.
- **The bundled library snapshot refuses a blob written before the cross-feature symbol kind
  existed.** Adding the kind renumbered the symbol kinds after it, which the snapshot stores as
  integers, so a snapshot from an older build could have been decoded with the wrong kinds; the
  format version now moves with the numbering and a test pins it.
