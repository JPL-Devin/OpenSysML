package main

import (
	"flag"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/usage"
)

// doc describes the command for both the terminal help and the man page, so a
// mode documented for one is documented for the other.
func doc() usage.Doc {
	return usage.Doc{
		Command:    "sysml",
		ManSection: 1,
		Summary:    "run, check, convert and render SysML v2 and KerML models",
		Synopsis:   []string{"[options] [file...]"},
		Description: []string{
			"sysml loads the models it is given — a file, a directory to walk or a " +
				"glob — as a single model, so a declaration in one file resolves " +
				"against the others whichever order they were named in. With no " +
				"expression or check to carry out it opens an interactive prompt; " +
				"otherwise it does what was asked and exits on the verdict, which " +
				"is what lets a run gate a build.",
		},
		Sections: []usage.Section{{
			Title: "Examples",
			Examples: []usage.Example{
				usage.Ex("sysml", "Start interactive REPL"),
				usage.Ex(`sysml -e "5 + 3"`, "Evaluate and exit"),
				usage.Ex(`sysml -e "expr" file.sysml`, "Load file, evaluate, and exit"),
				usage.Ex("sysml file.sysml", "Load file and start REPL"),
				usage.Ex("sysml -debug file.sysml", "Load file, reporting every diagnostic"),
				usage.Ex("sysml -trace file.sysml", "Load file, reporting each execution step"),
			},
		}, {
			Title: "Checking a model",
			Examples: []usage.Example{
				usage.Ex("sysml -constraint MassBudget model.sysml", "Evaluate one constraint and exit"),
				usage.Ex("sysml -requirement PowerMargin model.sysml", "Evaluate one requirement and exit"),
				usage.Ex("sysml -satisfy model.sysml", "Evaluate every satisfaction assertion"),
				usage.Ex("sysml -satisfy=Ctx model.sysml", "...only the ones Ctx states"),
				usage.Ex("sysml -instantiate p -constraint C model.sysml", "Check C against an object of p"),
				usage.Ex("sysml -validate model.sysml", "Report diagnostics only"),
				usage.Ex("sysml -validate -strict model.sysml", "...asking whether it is conforming SysML v2"),
				usage.Ex(`sysml -calc "Fall(3, 4)" model.sysml`, "Invoke a calculation"),
				usage.Ex(`sysml -run-query "Heavy root=scope" model.sysml`, "Execute a document query"),
				usage.Ex("sysml -action Drive model.sysml", "Run an action to completion"),
				usage.Ex("sysml -state Mission -advance 10 model.sysml", "Run a state machine for 10 time units"),
				usage.Ex("sysml -satisfy -json model.sysml", "Report the verdicts as JSON"),
			},
			Paragraphs: []string{"Each check flag may be repeated."},
		}, {
			Title: "Conversion",
			Examples: []usage.Example{
				usage.Ex("sysml model.sysml -convert ttl", "SysML notation to RDF Turtle, on stdout"),
				usage.Ex("sysml model.ttl -convert sysml", "RDF Turtle to SysML notation"),
				usage.Ex("sysml model.sysml -convert ttl -o m.ttl", "Write the conversion to a file"),
				usage.Ex("sysml in.txt -convert ttl -from sysml", "Name the input format explicitly"),
			},
			Paragraphs: []string{
				"The input format is taken from the file extension (.sysml, .kerml, " +
					".ttl) unless -from names it. Converting to the format it is " +
					"already in rewrites the input: notation is reformatted, Turtle " +
					"is normalized.",
				// Printed rather than restated, so the help cannot drift from what a
				// conversion reports.
				export.ExperimentalNotice,
				"Every run that converts RDF says so on stderr. Saving to .sysml or " +
					".kerml is stable.",
			},
		}, {
			Title: "Rendering a view",
			Examples: []usage.Example{
				usage.Ex("sysml model.sysml -render Views::vehicleView", "ASCII text at a terminal"),
				usage.Ex("sysml model.sysml -render Views::vehicleView -render-form markdown", ""),
				usage.Ex("sysml model.sysml -render Views::vehicleView -o view.mmd", ""),
				usage.Ex("sysml model.sysml -render-all rendered", ""),
			},
			Paragraphs: []string{
				"The rendering is the one the view's render member states, and a " +
					"containment tree where it states none. It is tool-defined " +
					"output: SysML v2 specifies the notation, not how a tool draws " +
					"it. Notices — an empty view, an element the rendering cannot " +
					"represent — go on stderr.",
			},
		}, {
			Title: "Rendering a document",
			Examples: []usage.Example{
				usage.Ex("sysml model.sysml -render-document Reports::MassReport", "Markdown on stdout"),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -o report.md", ""),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -doc-form pdf -o report.pdf", ""),
				usage.Ex("sysml model.sysml -render-documents rendered", "every document, linked"),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -doc-form pdf "+
					"-pdf-engine pandoc -pdf-title-page -pdf-toc -pdf-number-sections -o report.pdf", ""),
			},
			Paragraphs: []string{
				"A document is a part def specializing DocumentQueries::Document. Its " +
					"queries are bound in the model and run against it, and the " +
					"result is written as CommonMark-compatible Markdown.",
				"-doc-form pdf converts that Markdown with an external converter named " +
					"by -pdf-engine — weasyprint (default), pandoc or prince — run as " +
					"a subprocess, never linked in; diagrams are pre-rendered to SVG " +
					"with mermaid-cli (mmdc). None of these tools is needed until PDF " +
					"output is asked for; scripts/download-doc-pdf-toolchain.sh " +
					"provisions pinned copies.",
			},
		}, {
			Title: "Flag order",
			Paragraphs: []string{
				"Flags may be written before or after the model they apply to. A file " +
					"named like a flag is read as a file after --, which ends the " +
					"flags: sysml -trace -- -m.sysml",
			},
		}, {
			Title: "Reading from standard input",
			Examples: []usage.Example{
				usage.Ex("cat model.sysml | sysml -validate -", "A lone - names standard input"),
				usage.Ex("cat model.sysml | sysml - -convert ttl -from sysml", ""),
			},
			Paragraphs: []string{
				"What was read from standard input is called <stdin> in diagnostics, " +
					"and a file really named \"-\" is read by naming it ./- instead.",
			},
		}, {
			Title: "Profiling a run",
			Examples: []usage.Example{
				usage.Ex("sysml -validate -memstats model.sysml", "Report what the run cost, on stderr"),
				usage.Ex("sysml -validate -memprofile heap.out model.sysml", "Write a heap profile for go tool pprof"),
				usage.Ex("sysml -validate -cpuprofile cpu.out model.sysml", "Write a CPU profile for go tool pprof"),
			},
		}, {
			Title: "Exit status",
			Lead:  []string{"Every run that is not a prompt exits:"},
			Items: []usage.Item{
				usage.Entry("0", "It did what was asked."),
				usage.Entry("1", "The model answered false for a check."),
				usage.Entry("2", "What was asked could not be carried out at all — an unreadable "+
					"file, a model that did not analyse cleanly, an unresolved name, a "+
					"failed conversion."),
			},
		}, {
			Title: "Output streams",
			Paragraphs: []string{
				"What was asked for is reported on stdout and what went wrong on " +
					"stderr, prefixed \"sysml: \" unless it locates a finding in the " +
					"source.",
			},
		}, {
			Title:      "Environment",
			ManOnly:    true,
			Items:      append(usage.BudgetEnvironment(), solverEnvironment()...),
			Paragraphs: []string{usage.LegacyPrefixNote, usage.BudgetScopeNote},
		}, {
			Title:   "Files",
			ManOnly: true,
			Items: []usage.Item{
				usage.Entry("$XDG_STATE_HOME/sysml/history", "Prompt history, when XDG_STATE_HOME is set and writable."),
				usage.Entry("~/.sysml_history", "Prompt history otherwise. When neither can be written the history is kept for the session only."),
			},
		}, {
			Title:   "Reporting bugs",
			ManOnly: true,
			Paragraphs: []string{
				"Report bugs at https://github.com/Open-MBEE/OpenSysML/issues.",
			},
		}},
		SeeAlso: []string{"sysml-lsp(1)", "sysml-grpc(1)", "https://github.com/Open-MBEE/OpenSysML"},
	}
}

// solverEnvironment describes the variables of the experimental solving
// extension, which only the prompt's solver commands read.
func solverEnvironment() []usage.Item {
	return []usage.Item{
		usage.Entry("OPENSYSML_SMT", "Executable the %check, %explain, %solve, %configure and %optimize commands drive as their SMT solver, speaking SMT-LIB2 on standard input (experimental). Unset looks for z3, then cvc5, on PATH."),
		usage.Entry("OPENSYSML_SMT_TIMEOUT", "How long one solver query may take, as a Go duration. Default 10s, after which the verdict is unknown."),
		usage.Entry("OPENSYSML_SMT_CORE_BUDGET", "How long %explain may spend reducing an unsat core to a minimal one, as a Go duration. Default 30s."),
		usage.Entry("OPENSYSML_SMT_MAX_CONFIGURATIONS", "How many variant selections %configure ... all may report before saying the enumeration was cut short. Default 32."),
	}
}

// registerFlags declares the command's flags on fs, so the help, the man page
// and a run all read one declaration of each.
func registerFlags(fs *flag.FlagSet) {
	fs.BoolVar(&showHelp, "help", false, "Show this help and exit")
	fs.BoolVar(&showHelp, "h", false, "Show this help (shorthand)")
	fs.BoolVar(&showMan, "man", false, "Write this command's manual page, in roff, to stdout and exit")
	fs.Var(&evalExprs, "eval", "Evaluate expression and exit (can be specified multiple times)")
	fs.Var(&evalExprs, "e", "Evaluate expression and exit (shorthand)")
	fs.BoolVar(&showVersion, "version", false, "Show version information")
	fs.BoolVar(&showVersion, "v", false, "Show version (shorthand)")
	fs.BoolVar(&debugMode, "debug", false, "Report every diagnostic over the whole session buffer, with the pass that produced it")
	fs.BoolVar(&quietMode, "quiet", false, "Report errors only, suppressing warnings")
	fs.BoolVar(&strictMode, "strict", false, "Judge the model as conforming SysML v2: notation no pinned production admits is an error, not a warning")
	fs.BoolVar(&traceMode, "trace", false, "Report each execution step: expression evaluation, calc invocation, action tokens, state transitions")
	fs.StringVar(&convertFormat, "convert", "", "Convert the model to this format instead of running it: sysml, kerml, ttl, turtle or rdf (RDF is experimental)")
	fs.StringVar(&queryText, "query", "", "Evaluate OSLC Query text against the model instead of running the REPL")
	fs.StringVar(&outputPath, "output", "", "Write conversion output to this file (default: stdout)")
	fs.StringVar(&outputPath, "o", "", "Write conversion output to this file (shorthand)")
	fs.StringVar(&fromFormat, "from", "", "Input format for -convert: sysml, kerml, ttl, turtle or rdf (default: from the input's extension)")
	fs.StringVar(&renderView, "render", "", "Render this view of the model instead of running it, in the form its render member states")
	fs.StringVar(&renderAllDir, "render-all", "", "Render every declared view into this directory")
	fs.StringVar(&renderDoc, "render-document", "", "Compile this document definition, run its queries and write the rendered Markdown")
	fs.StringVar(&renderDocsDir, "render-documents", "", "Render every document definition as linked Markdown into this directory")
	fs.StringVar(&renderForm, "render-form", "", "Form -render or -render-all writes: text, mermaid or markdown (default: destination-dependent for -render, each kind's machine form for -render-all)")
	fs.StringVar(&docForm, "doc-form", "", "Form -render-document writes: markdown (default) or pdf, which drives an external converter")
	fs.StringVar(&pdfEngine, "pdf-engine", "", "Converter -doc-form pdf drives: weasyprint (default), pandoc or prince")
	fs.BoolVar(&pdfTitlePage, "pdf-title-page", false, "Put the document title on a page of its own (-doc-form pdf)")
	fs.BoolVar(&pdfTOC, "pdf-toc", false, "Write a table of contents ahead of the content (-doc-form pdf)")
	fs.BoolVar(&pdfNumbering, "pdf-number-sections", false, "Number the section headings hierarchically (-doc-form pdf)")
	fs.Var(&deprecatedFlag{instead: "-to has been replaced by -convert, as `sysml model.sysml -convert ttl`"}, "to", "Replaced by -convert, which names the output format")
	fs.Var(&modelChecks.instantiate, "instantiate", "Create an object of this definition before the checks, so a verdict is about it (repeatable)")
	fs.Var(&modelChecks.constraints, "constraint", "Evaluate this constraint and exit (repeatable)")
	fs.Var(&modelChecks.requirements, "requirement", "Evaluate this requirement and exit (repeatable)")
	fs.Var(&modelChecks.satisfy, "satisfy", "Evaluate every satisfaction assertion, or with -satisfy=<name> those the named element states (repeatable)")
	fs.BoolVar(&modelChecks.validate, "validate", false, "Analyse the model and report its diagnostics, exiting nonzero on an error")
	fs.Var(&modelChecks.calcs, "calc", "Invoke this calculation and report what it computed, as -calc \"Fall(3, 4)\" (repeatable)")
	fs.Var(&modelChecks.queries, "run-query", "Execute this document query and report its rows, as -run-query \"HeavySubsystems root=telescope\" (repeatable)")
	fs.Var(&modelChecks.actions, "action", "Run this action to completion, as -action \"Drive rover1\" to run it on an object (repeatable)")
	fs.Var(&modelChecks.states, "state", "Run this state machine, as -state \"Mission rover1\" to run it on an object (repeatable)")
	fs.Var(&modelChecks.advance, "advance", "Simulated time units to run each -state machine for (default: only its initial transition)")
	fs.BoolVar(&modelChecks.jsonOut, "json", false, "Report checks as one JSON document rather than as lines")
	fs.StringVar(&cpuProfilePath, "cpuprofile", "", "Write a CPU profile of the run to this file, for go tool pprof")
	fs.StringVar(&memProfilePath, "memprofile", "", "Write a heap profile of the run to this file, for go tool pprof")
	fs.BoolVar(&memStats, "memstats", false, "Report on stderr what the run cost: wall time, memory allocated, memory taken from the OS")
}

// docFlags is a flag set holding the command's flags, for rendering the help or
// the man page without a command line to parse.
func docFlags() *flag.FlagSet {
	fs := flag.NewFlagSet("sysml", flag.ContinueOnError)
	registerFlags(fs)
	return fs
}
