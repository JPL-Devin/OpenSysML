# 3. From the command line

Every check the REPL performs is also a flag, so a model can be checked from a script, a
Makefile or a CI job. This chapter is the narrative; every flag and exit status is tabulated in
[reference/cli.md](../reference/cli.md).

The same checks the REPL runs are available as flags, so a model can be checked
from a script or a build step without a prompt. Take `checks.sysml`:

```sysml
package MyModel {
    part def Sensor {
        attribute reading = 0.0;
        attribute threshold = 100.0;
    }

    requirement def ReadingRequirement {
        subject sensor : Sensor;
        require sensor.reading <= sensor.threshold;
    }
    requirement healthy : ReadingRequirement;

    part hot : Sensor {
        attribute :>> reading = 140.0;
        constraint inRange { reading <= threshold }
    }
    part cold : Sensor {
        attribute :>> reading = 20.0;
        constraint inRange { reading <= threshold }
    }

    part checks {
        assert satisfy healthy by cold;
        assert satisfy healthy by hot;
    }

    calc def Margin {
        in reading;
        in threshold;
        return threshold - reading;
    }

    action calibrate {
        attribute offset = 0.0;
        first start;
        action adjust {
            assign offset := offset + 1.5;
        }
        done end;
        then start adjust;
        then adjust end;
    }

    state Monitor {
        initial off;
        state warming {
            accept after 10 then running;
        }
        state running;
        off then warming;
    }
}
```

Each flag may be repeated, and `-instantiate` runs first — whatever order the
flags are written in — so the verdicts after it are about that object:

```bash
$ sysml -instantiate MyModel::cold -constraint MyModel::cold::inRange checks.sysml
✓ package MyModel
✓ Created instance of MyModel::cold
  ID: 1
  Use %features MyModel::cold to inspect
✓ Constraint MyModel::cold::inRange passed (on MyModel::cold ID: 1)

$ sysml -satisfy checks.sysml
✓ package MyModel
✓ satisfy healthy by cold holds (on MyModel::cold ID: 1)
✗ satisfy healthy by hot fails (on MyModel::hot ID: 2)
  Required condition evaluated to false: sensor.reading <= sensor.threshold
```

Every check flag is listed in
[reference/cli.md § Command Reference](../reference/cli.md#command-reference):
one constraint or requirement by name, every satisfaction assertion the model
states, a calculation, an action, a state machine, and `-json` for a machine-readable
report.

The exit status is what a build step gates on, and it is the same on every run
that is not a prompt — an evaluation, a conversion, a plain load — not only on a
check: `0` when what was asked for was done, `1` when the model answered false,
`2` when nothing was decided. The whole contract is in
[reference/cli.md § Exit status](../reference/cli.md#exit-status).

Status 2 is kept apart from 1 because an undecided check is not evidence against
the model — treat it as a broken check, not a failing one. A condition that
evaluated to false is the model's own answer; a name that does not resolve, or a
condition over a feature with no value, is not:

```bash
$ sysml -constraint MyModel::hot::inRange -instantiate MyModel::hot checks.sysml; echo "exit=$?"
✓ package MyModel
✓ Created instance of MyModel::hot
  ID: 1
  Use %features MyModel::hot to inspect
✗ Constraint MyModel::hot::inRange failed (on MyModel::hot ID: 1)
  Assertion evaluated to false: reading <= threshold
exit=1

$ sysml -constraint MyModel::nosuch checks.sysml; echo "exit=$?"
✓ package MyModel
error: unresolved reference: MyModel::nosuch
exit=2

$ sysml -requirement MyModel::healthy checks.sysml; echo "exit=$?"
✓ package MyModel
? Requirement MyModel::healthy could not be evaluated
  Error: requirement healthy: require condition evaluation failed: no value for feature sensor
exit=2
```

That last case is the shape of a requirement with a `subject`: nothing binds the
subject, so it has no object to be about. Write the intended pair into the model
as `assert satisfy healthy by hot;` and check it with `-satisfy`, which creates
the subject itself — `-requirement` decides a requirement whose conditions stand
on their own, or one carried by a part an `-instantiate` created.

A verdict is written to stdout and an undecided check to stderr, as is every
other finding — diagnostics and warnings included — so
`sysml -satisfy checks.sysml > verdicts.txt` keeps the results and leaves what
went wrong on the terminal.

## Checking as a gate

`-validate` asks nothing of the model's conditions: it loads it and reports what
analysis found, which is the lint step to run before any verdict is trusted.

```bash
$ sysml -validate checks.sysml; echo "exit=$?"
✓ package MyModel
✓ checks.sysml: no errors
exit=0

$ sysml -validate bad.sysml; echo "exit=$?"   # a file with a syntax error in it
bad.sysml:2:45: error: expected an expression
    part def Battery { attribute capacity = ; }
                                            ^
sysml: bad.sysml did not analyse cleanly; no check was made
exit=2
```

A lone `-` names standard input wherever a file is taken, so a model can be
piped in; its diagnostics are counted from `<stdin>`. A file really called `-` is
read by naming it `./-`, and `-convert` needs `-from` for piped input because the
stream carries no extension to take the format from.

```bash
$ cat checks.sysml | sysml -validate -
✓ package MyModel
✓ <stdin>: no errors

$ cat checks.sysml | sysml - -convert ttl -from sysml > model.ttl
```

Every check mode is gated the same way, so a model with an error never reports a
verdict about itself — a condition read out of a model the tool could not fully
read would be an answer about a different model than the one you wrote. Name as
many files as the model spans, in any order: the gate is about the model as a
whole, so a reference from one file to a declaration in another resolves.

## Running behavior

The debuggers have non-interactive forms that run to completion and report the
values they produced. `-calc` takes the invocation, and `-action` and `-state`
take the behavior's name optionally followed by the object performing it:

```bash
$ sysml -calc "MyModel::Margin(20.0, 100.0)" checks.sysml
✓ package MyModel
✓ MyModel::Margin(20.0, 100.0)
  = 80.00

$ sysml -action MyModel::calibrate checks.sysml
✓ package MyModel
✓ Started action executor for "MyModel::calibrate"
  State: Running
  Tokens: 1
✓ Action completed
  Final state: Completed
  Results:
    offset = 1.50

$ sysml -state MyModel::Monitor -advance 15 checks.sysml
✓ package MyModel
✓ Started state machine executor for "MyModel::Monitor"
  Current state: off
  Time: 0.00
  Events: 1
✓ Advanced to 15.00 (2 event(s) processed)
  Current state: running
  Last event at: 10.00
  Remaining events: 0
```

A state machine only takes its initial transition unless `-advance` says how much
simulated time to run it for; `-advance 0` runs it to the present, dispatching what
is already due, and `-advance` with no `-state` to run is reported as a misuse
rather than silently dropped. An action that stopped short of completing
— a deadlock, or the step budget reached — is reported as a check that was never
decided, i.e. status 2, since it produced no outputs to judge.

## Machine-readable results

`-json` reports the same run as one document, so a build step reads structure
rather than parsing `✓`/`✗`. It reports the checks, not the model: use `-convert`
to serialize the model itself.

```bash
$ sysml -satisfy -json checks.sysml; echo "exit=$?"
{
  "status": "fails",
  "exit": 1,
  "checks": [
    {
      "subject": "satisfy healthy by cold",
      "status": "holds",
      "values": null,
      "lines": [
        "✓ satisfy healthy by cold holds (on MyModel::cold ID: 1)"
      ]
    },
    {
      "subject": "satisfy healthy by hot",
      "status": "fails",
      "values": null,
      "lines": [
        "✗ satisfy healthy by hot fails (on MyModel::hot ID: 2)",
        "  Required condition evaluated to false: sensor.reading \u003c= sensor.threshold"
      ]
    }
  ],
  "diagnostics": null,
  "output": [
    "✓ package MyModel"
  ],
  "errors": null
}
exit=1
```

`status` is the worst verdict reached and `exit` the status the process exits
with. A calculation's or a machine's values are reported as `values` entries, the
findings of analysis as `diagnostics` — the warnings of a model that analyses
cleanly as well as the errors of one that does not, each with the `file` it is in
and its `line` and `column` there — and whatever stopped a check from being made as
`errors`, so the whole document goes to stdout and nothing needs to be read off
stderr.

## Running from a script

`-e` evaluates without entering the prompt, so a model can be queried from a
shell:

```bash
$ sysml -e "RdfInteropDemo::Rover::mass" examples/rdf-interop-demo.sysml
✓ package RdfInteropDemo
✓ RdfInteropDemo::Rover::mass
  = 899.00
```

Two things matter before a pipeline depends on it:

- **What was asked for is on stdout and what went wrong on stderr.** Evaluated
  values, conversion output and verdict lines are results; a model's diagnostics
  and warnings, a failed evaluation, a file that could not be read and the
  `wrote <file> …` note of a successful `-convert -o` are not, so that stdout
  carries the conversion alone.
- **The exit status says whether the model answered what was asked**: `0` it did,
  `1` it answered false, `2` it answered nothing. A warning leaves the status `0`.
  The status codes are documented once, in
  [reference/cli.md § Exit status](../reference/cli.md#exit-status),
  with a CI recipe that gates on them.

---

Next: [4. The REPL](04-repl.md).
