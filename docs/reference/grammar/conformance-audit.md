# Grammar Conformance Audit — reserved words and state/action notation

This is the reviewable claim behind two findings of the
[pilot differential](../../project/pilot-differential.md) — that we reserved
words the grammars do not, and that we rejected state-machine notation they
admit: every word we reserve and every state-machine construct we accept,
checked against the pinned OMG grammars, with the resulting policy.

## Ground truth

The grammars are the ones at the pin in `scripts/pilot-pin.sh`
(`PILOT_TAG=2026-05`, `Systems-Modeling/SysML-v2-Pilot-Implementation`), read
from a sparse clone rather than vendored:

- `org.omg.kerml.xtext/src/org/omg/kerml/xtext/KerML.xtext` — cited as `KerML.xtext`
- `org.omg.sysml.xtext/src/org/omg/sysml/xtext/SysML.xtext` — cited as `SysML.xtext`
- `org.omg.kerml.expressions.xtext/src/org/omg/kerml/expressions/xtext/KerMLExpressions.xtext` — cited as `KerMLExpressions.xtext`

A word is *standard* when it appears as a quoted literal in one of those files,
in the position we accept it. Line numbers are at the pin.

## Verdict per word

All eleven words below appear as a literal in **none** of the three grammars, at
any line: they are not notation OMG defines, so reserving them only stopped
models from using them as names. Each is now an ordinary name, matched
contextually where our own notation needs it — the treatment `point`, `on` and
`var` already get.

| Word | `KerML.xtext` | `SysML.xtext` | `KerMLExpressions.xtext` | Verdict |
|------|---------------|---------------|--------------------------|---------|
| `choice` | absent | absent | absent | unreserve; notation is an OpenSysML extension (warning) |
| `decision` | absent | absent | absent | unreserve; **an ordinary name only** — the action node spelled `decision` is no longer accepted, write `decide` |
| `deep` | absent | absent | absent | unreserve; notation is an OpenSysML extension (warning) |
| `defer` | absent | absent | absent | unreserve; notation is an OpenSysML extension (warning) |
| `done` | absent | absent | absent | unreserve; **silent** — see "`done` is a library name, not notation" |
| `final` | absent | absent | absent | unreserve; state notation (warning); the action node spelled `final` is no longer accepted, write `done` |
| `history` | absent | absent | absent | unreserve; notation is an OpenSysML extension (warning) |
| `initial` | absent | absent | absent | unreserve; state notation (warning); the action node spelled `initial` is no longer accepted, write `first` |
| `junction` | absent | absent | absent | unreserve; notation is an OpenSysML extension (warning) |
| `region` | absent | absent | absent | unreserve; notation is an OpenSysML extension (warning) |
| `shallow` | absent | absent | absent | unreserve; notation is an OpenSysML extension (warning) |

`done` is the acceptance test that unreserving worked: the bundled normative
library declares features named `done` (`Systems Library/Actions.sysml:50`,
`Items.sysml:34`, `Parts.sysml:29`, `States.sysml:34`, `UseCases.sysml:28`) and
references them (`Flows.sysml:57`, `:69`). We reported an error on every one.

### `done` is a library name, not notation

The OMG corpora write `then done;` (`Systems Library/Actions.sysml:230`;
training examples `17. Control/Fork Join Example.sysml:39`, `Decision
Example.sysml:32`, `Control Structures Example.sysml:27`, `35. Use Cases/Use
Case Usage Example.sysml:35`) and `snapshot junked = done;`
(`27. Occurrences/Time Slice and Snapshot Example.sysml:25`). Those are plain
references to `Actions::Action::done`, a feature of the standard library, not a
keyword. Our parser still reads `done;` and `then done;` as the final node it
always built, and the construct stays **silent**: warning on it would warn on
OMG-authored files, which the classification forbids. `final` was our own
spelling of the same node and is no longer accepted as one.

## Verdict per construct

Positions checked against `SysML.xtext` `StateBodyItem` (1755-1770),
`StateDefBody` (1744-1746), `StateUsageBody` (1836-1838), `TransitionUsage`
(1851-1880), and the action node productions (1666-1730).

### Standard — silent

| Construct | Citation |
|-----------|----------|
| `entry <action>` state subaction | `SysML.xtext:1772-1778` (`EntryActionMember`, `EntryActionKind : 'entry'`) |
| `do <action>` state subaction | `SysML.xtext:1780-1786` (`DoActionMember`, `DoActionKind : 'do'`) |
| `exit <action>` state subaction | `SysML.xtext:1788-1794` (`ExitActionMember`, `ExitActionKind : 'exit'`) |
| `parallel` before a state body | `SysML.xtext:1745`, `:1837` (`isParallel ?= 'parallel'`) |
| `then <target>;` succession | `SysML.xtext:1705`, `:1711`, `:1724`, `:1799`; `KerML.xtext:894` |
| `first <source>` | `SysML.xtext:1385`, `:1720`, `:1855`; `KerML.xtext:893` |
| `done;` / `then done;` final node | no literal; a reference to `Actions::Action::done`, written by the OMG corpora (above) |
| `fork <name>;`, `join <name>;`, `merge <name>;`, `decide <name>;` | `SysML.xtext:1684`, `:1678`, `:1666`, `:1672`; admitted in a state body by `StateBodyItem` → `BehaviorUsageMember` (`SysML.xtext:1761-1763`) |
| `accept <trigger> [via <port>]` | `SysML.xtext:1447`, `:1894` (`trigger = 'accept'`), `via` at `:1450` |
| `when <expr>` trigger | `SysML.xtext:1483-1485` (`ChangeTriggerKind : 'when'`) |
| `transition [<name>] first <src> … then <tgt>;` | `SysML.xtext:1851-1880` (`TransitionUsage`) |
| `state <name>;` / `state <name> { … }` | `SysML.xtext:1733`, `:1741`, `:1833` |
| `send`, `terminate`, `assign`, `perform`, `if`/`else`, `while`, `loop`, `for` | `SysML.xtext:1500`, `:1643`, `:1540`, `:1412`, `:1600` and the loop node productions |
| `namespace N;` / `namespace N { … }` in a `.kerml` file | `KerML.xtext:119`, `:124-125` (`'namespace' Identification?`), `:128` (`NamespaceBody : ';' \| '{' … '}'`) |

### OpenSysML extension — warning `nonstandard-notation`

No production admits these anywhere in the pinned grammars. They stay parsed —
deleting notation users already write is worse than diagnosing it — and are
reported as a warning by default. Under the opt-in
[strict conformance mode](../../guide/03-command-line.md#strict-conformance) the
same findings are errors: strictness changes the severity of this table's
diagnostics and nothing else, so the tree, the spans and the messages are the
ones below in either mode. The mode is what answers "is this file conforming
SysML v2?"; the default mode's acceptance of these constructs is intended and
unchanged.

| Construct | Why it is not standard |
|-----------|------------------------|
| `initial <name>;` in a state body | `StateBodyItem` (`SysML.xtext:1755-1770`) has no such member; the standard way to mark the first state is `entry; then <state>;` (`EntryTransitionMember`, `:1796-1801`) |
| `final <name>;` in a state body | same; no `final` literal anywhere |
| `region <name> { … }` | no `region` literal; the standard orthogonality marker is `parallel` (`:1745`) |
| `choice <name>;`, `junction <name>;` | no literal; no pseudostate production of any kind |
| `history <name>;`, `shallow history <name>;`, `deep history <name>;` | same |
| `entry point <name>;`, `exit point <name>;` | `entry`/`exit` are literals only as state subaction kinds (`:1777`, `:1793`); no `point` literal exists |
| `defer <event> [, <event>]*;` | no `defer` literal; `StatePerformance::deferrable` has the semantics but no notation |
| `transition [<name>] <src> to <tgt>;` | `to` is a literal (`SysML.xtext:1077`, `:1168`, `:1253`, `:1287`; `KerML.xtext:838`, `:1009`) but in connector, interface, message and flow ends only — `TransitionUsage` (`:1851-1880`) states its ends with `first` and `then` |

### KerML-only notation in a `.sysml` file — warning `kerml-notation`

`namespace` is a literal in `KerML.xtext` only (`:125`); `SysML.xtext` has none.
A `.sysml` file's root is `RootNamespace : PackageBodyElement*`
(`SysML.xtext:38`), which admits package members, element filters, aliases and
imports — not a `namespace` declaration. Both spellings, `namespace N;` and
`namespace N { … }`, are legal KerML, so the semicolon form is not the defect:
the defect is the production being used in a SysML file at all. It stays parsed
and is warned in `.sysml`, silent in `.kerml`.

### Reserved by SysML only — a name in a `.kerml` file

A word is reserved by the grammar of the file it is written in, so a literal of
`SysML.xtext` alone is an ordinary name in KerML. These four are unreserved by
file kind; they keep their SysML meanings in `.sysml`, where the literal exists.

| Word | `KerML.xtext` | `KerMLExpressions.xtext` | `SysML.xtext` | Corpus witness |
|------|---------------|--------------------------|---------------|----------------|
| `at` | absent | absent | `:1480` (`TimeTriggerKind`) | `expr at { … }`, `Variable Feature Examples/Enhancements/ExtendedOccurrences.kerml:16` |
| `while` | absent | absent | `:1617` (`WhileLoopActionUsage`) | `expr while { … }`, same file `:25` |
| `merge` | absent | absent | `:1666` (`MergeNode`) | `member step merge : …`, `Enhancements/TimeVaryingSteps.kerml:4`, imported at `:6` |
| `decide` | absent | absent | `:1672` (`DecisionNode`) | `member step decide : …`, same file `:25`, imported at `:27` |

`featured by` is the converse case: `TypeFeaturingPart` (`KerML.xtext:569-571`)
and `OwnedTypeFeaturing` (`:659`) are KerML productions with no SysML
counterpart, so the clause is parsed everywhere and warned as `kerml-notation`
in a `.sysml` file — the same treatment `namespace` gets in the other direction.

## Judgment calls

Everything above is a grammar citation except these, recorded so a reviewer can
disagree with them:

- **`done` silent, `final` warned.** No grammar has either literal. `done` is
  classified standard because the OMG corpora write it (as a library-feature
  reference) and a warning there would be a false positive; `final` appears in
  no OMG file. `final` is warned as a **state** marker only: the action node it
  once spelled is no longer accepted at all.
- **An alias of a standard node is removed, not warned.** The action nodes
  spelled `initial`, `final` and `decision` were pure aliases of `first`, `done`
  and `decide`, so they are gone rather than diagnosed; each word stays an
  ordinary name.
- **State-body `fork`/`join` silent.** They are action node literals, and
  `StateBodyItem` admits a `BehaviorUsageMember`, so we read them as standard in
  a state body even though the pilot's state examples do not use them there.
- **`transition <src> to <tgt>` warned.** `to` is a literal, but not in any
  transition production, so the construct is ours.
