- **`%state <machine> <object>` attaches to an exhibited machine that states its own body
  under the definition's type.** For `exhibit state m : Mission { state extra; }`, `%state Mission
  tank` used to report that the object "exhibits no running machine of this kind" and start a
  second, detached performance of `Mission`, and `%state Mission` alone found no exhibitor. The
  machine an object exhibits is now addressable by every definition its bindings conform to —
  the one typing the usage and the ones that definition specializes — whether the body lives in
  the usage or in the definition, so both forms attach to the running machine as the reference
  already described. The `-state` command-line flag shares the path.
