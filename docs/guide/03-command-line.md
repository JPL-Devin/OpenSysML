# 3. From the command line

Every check the REPL performs is also available as a command-line flag, so you can check a model
from a script, a Makefile or a continuous integration job without a prompt. This
chapter explains the workflow; every flag and exit status is listed in
[reference/cli.md](../reference/cli.md).

The examples below use the following model, `checks.sysml`:

```sysml
package MyModel {
    part def Sensor {
        attribute reading = 0.0;
        attribute threshold = 100.0;
    }

    requirement def ReadingRequirement {
        subject sensor : Sensor;
        require constraint {
            sensor.reading <= sensor.threshold
        }
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
        threshold - reading
    }

    action calibrate {
        attribute offset = 0.0;
        first start;
        then action adjust {
            assign offset := offset + 1.5;
        }
        then done;
    }

    state Monitor {
        entry;
        then off;
        state off;
        state warming {
            accept after 10 then running;
        }
        state running;
        transition first off then warming;
    }
}
```

Each flag may be repeated. `-instantiate` always runs first, whatever order the flags are
written in, so the verdicts that follow apply to the object it created:

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
[reference/cli.md § Command Reference](../reference/cli.md#command-reference). You can check
one constraint or requirement by name, every satisfaction assertion the model
states, a calculation, an action, or a state machine, and add `-json` for a machine-readable
report.

Build steps can gate on the exit status. It follows the same rules for every non-interactive
run, whether that is a check, an evaluation, a conversion or a plain load: `0` when the requested
operation succeeded, `1` when the model answered false, and `2` when nothing was decided. The full
contract is in [reference/cli.md § Exit status](../reference/cli.md#exit-status).

Status 2 is kept separate from status 1 because an undecided check is not evidence against the
model: treat it as a broken check rather than a failing one. A condition that
evaluates to false is the model's own answer; an unresolved name, or a condition over a feature
that has no value, is not:

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
sysml: unresolved reference: MyModel::nosuch
exit=2

$ sysml -requirement MyModel::healthy checks.sysml; echo "exit=$?"
✓ package MyModel
? Requirement MyModel::healthy could not be evaluated
  Error: requirement healthy: sensor subject is unbound: bind it (`subject sensor = <element>`), check it on an object, or assert `satisfy healthy by <element>`
exit=2
```

The last case shows a requirement that declares a `subject` that nothing binds,
so the requirement has no object to evaluate against. Declare the intended pairing in the model
as `assert satisfy healthy by hot;` and check it with `-satisfy`, which creates the subject
itself. `-requirement` is for a requirement whose conditions stand on their own, or one
carried by a part that `-instantiate` created.

Verdicts go to standard output. Undecided checks and everything else, including diagnostics
and warnings, go to standard error. So
`sysml -satisfy checks.sysml > verdicts.txt` records the results in the file and leaves any
problems visible on the terminal.

## Checking as a gate

`-validate` does not evaluate any of the model's conditions. It loads the model and reports
what analysis found. Run it as a lint step before relying on any verdict.

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

A single `-` stands for standard input wherever a file name is accepted, so you can pipe a model
in; its diagnostics are reported against `<stdin>`. To read a file that is actually named `-`,
write `./-`. `-convert` needs `-from` for piped input, because a stream has no file
extension to infer the format from.

```bash
$ cat checks.sysml | sysml -validate -
✓ package MyModel
✓ <stdin>: no errors

$ cat checks.sysml | sysml - -convert ttl -from sysml > model.ttl
```

Every check mode is gated the same way, so a model containing an error never reports a
verdict about itself: a condition read from a model the tool could not fully parse would
describe a different model from the one you wrote. Name as many files as the model
spans, in any order. The gate applies to the model as a whole, so a reference from one file to
a declaration in another resolves correctly.

## Strict conformance

OpenSysML accepts several notations of its own that no SysML v2 production admits: `defer`, and
the `choice`, `junction`, `history` and `entry`/`exit point` pseudostates. These are reported as
warnings, so a model that uses them still analyses cleanly. `-strict` promotes those warnings
to errors, which turns the run into a test of whether the file is conforming SysML v2.

The state machine below uses the `defer` extension so the difference is visible:

```sysml
package M {
    attribute def Alarm;
    state monitor {
        entry; then off;
        state off {
            defer Alarm;
        }
        state warming {
            accept after 10 then done;
        }
        succession first off then warming;
    }
}
```

```bash
$ sysml -validate monitor.sysml; echo "exit=$?"
monitor.sysml:6:13: warning: `defer <event>;` is an OpenSysML extension with no SysML v2 production: no notation states a deferred event
            defer Alarm;
            ^~~~~
✓ package M
✓ monitor.sysml: no errors
exit=0

$ sysml -strict -validate monitor.sysml; echo "exit=$?"
monitor.sysml:6:13: error: `defer <event>;` is an OpenSysML extension with no SysML v2 production: no notation states a deferred event
            defer Alarm;
            ^~~~~
sysml: monitor.sysml did not analyse cleanly; no check was made
exit=2
```

There is no standard notation for a deferred event, so a portable model marks the machine's
completion with `then done;`, as `warming` does above, and leaves out the `defer`. `-strict` does not
change what parses: the same file produces the same tree and the same findings in the same
places. Only their severity changes, and with it the exit status and the tier gate. It is a
portability check, so turn it on when another SysML v2 tool has to read the model and leave
it off otherwise. Each finding names the standard notation to use instead, and
[the conformance audit](../reference/grammar/conformance-audit.md) cites the grammar production
each extension is measured against. The same setting is available as `%strict` at the prompt
([4. The REPL](04-repl.md)), as the `sysml.strictConformance` editor setting
([8. Editors](08-editors.md)) and as `strict_conformance=True` from Python
([9. From your own program](09-python.md#from-python)).

## Running behavior

The debuggers have non-interactive forms that run to completion and report the values they
produce. `-calc` takes a call expression, while `-action` and `-state` take the name of the
behavior, optionally followed by the object performing it:

```bash
$ sysml -calc "MyModel::Margin(20.0, 100.0)" checks.sysml
✓ package MyModel
✓ MyModel::Margin(20.0, 100.0)
  = 80.0

$ sysml -action MyModel::calibrate checks.sysml
✓ package MyModel
✓ Started action executor for "MyModel::calibrate"
  State: Running
  Tokens: 1
✓ Action completed
  Final state: Completed
  Results:
    offset = 1.5

$ sysml -state MyModel::Monitor -advance 15 checks.sysml
✓ package MyModel
✓ Started state machine executor for "MyModel::Monitor"
  Current state: off
  Time: 0.0
  Events: 1
✓ Advanced to 15.0 (2 event(s) processed)
  Current state: running
  Last event at: 10.0
  Remaining events: 0
```

A state machine takes only its initial transition unless `-advance` says how much simulated
time to run for. `-advance 0` runs the machine up to the present, dispatching whatever is already
due. `-advance` without a matching `-state` is reported as a mistake rather than
silently ignored. An action that stops before completing, whether through deadlock or by
hitting the step budget, is reported as an undecided check (status 2), because it produced no
outputs to evaluate.

## Machine-readable results

`-json` reports the same run as a single JSON document, so a build step can read structured
data instead of parsing `✓` and `✗` markers. It reports the checks, not the model; use
`-convert` to serialize the model itself.

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

`status` is the worst verdict reached, and `exit` is the code the process exits with.
Values produced by a calculation or a state machine appear under `values`. What analysis
found appears under `diagnostics`, covering both the warnings of a model that analyses cleanly
and the errors of one that does not, each with the `file`, `line` and `column` where it
occurs. Anything that prevented a check from being made appears under `errors`. The whole
document goes to standard output, so nothing needs to be read from standard error.

## Running from a script

`-e` evaluates an expression without entering the prompt, so you can query a model from a
shell:

```bash
$ sysml -e "RdfInteropDemo::Rover::mass" examples/rdf-interop-demo.sysml
✓ package RdfInteropDemo
✓ RdfInteropDemo::Rover::mass
  = 899.0
```

Two things are worth knowing before a pipeline depends on this:

- **Results go to standard output and problems go to standard error.** Evaluated
  values, conversion output and verdict lines are results. A model's diagnostics and warnings,
  a failed evaluation, a file that could not be read, and even the `wrote <file> …` note of a
  successful `-convert -o` are not, so standard output carries the conversion alone.
- **The exit status says whether the model answered the question**: `0` if it did, `1`
  if it answered false, and `2` if it answered nothing. A warning leaves the status at `0`. The
  status codes are documented in
  [reference/cli.md § Exit status](../reference/cli.md#exit-status), along with a continuous
  integration recipe that gates on them.

---

Next: [4. The REPL](04-repl.md).
