package runtime

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Default bounds on one run. Each one stops a different kind of runaway, so each
// counts a different thing and has its own variable.
//
// The sizes are set by how long a runaway takes to report, not by memory:
// execution allocates nothing per step (peak RSS is ~34MB whether a run spends
// 10 thousand steps or 50 million), and the only thing a budget makes grow is a
// %trace, at 34-83 bytes an entry. Measured rates are ~13.6M evaluation steps/s
// and ~1.9M events/s, so these defaults report a runaway within about a second
// each, and a fully traced run at all four ceilings holds ~320MB.
const (
	// DefaultMaxSteps bounds expression evaluations.
	DefaultMaxSteps int64 = 10000000
	// DefaultMaxActionSteps bounds the token-flow steps one action run performs.
	DefaultMaxActionSteps int64 = 1000000
	// DefaultMaxStateEvents bounds the events one state machine run dispatches.
	DefaultMaxStateEvents int64 = 1000000
	// DefaultMaxDoSteps bounds the do activity actions one state machine run
	// performs.
	DefaultMaxDoSteps int64 = 5000000
)

// Environment variables overriding the defaults above, following the
// SYSML_LIBRARY_PATH convention.
const (
	MaxStepsEnvVar       = "SYSML_MAX_STEPS"
	MaxActionStepsEnvVar = "SYSML_MAX_ACTION_STEPS"
	MaxStateEventsEnvVar = "SYSML_MAX_EVENTS"
	MaxDoStepsEnvVar     = "SYSML_MAX_DO_STEPS"
)

// Budgets bounds one run of the runtime. The four bounds count incommensurable
// things — expression evaluations, action token-flow steps, state machine events
// and do activity actions — so raising one says nothing about the others.
type Budgets struct {
	MaxSteps       int64
	MaxActionSteps int64
	MaxStateEvents int64
	MaxDoSteps     int64
}

// DefaultBudgets returns the bounds a run uses when the environment names no
// override.
func DefaultBudgets() Budgets {
	return Budgets{
		MaxSteps:       DefaultMaxSteps,
		MaxActionSteps: DefaultMaxActionSteps,
		MaxStateEvents: DefaultMaxStateEvents,
		MaxDoSteps:     DefaultMaxDoSteps,
	}
}

// budgetVar is one bound: the variable that sets it, its default, and what the
// number counts (used to say what a rejected value should have been).
type budgetVar struct {
	env    string
	def    int64
	counts string
	field  func(*Budgets) *int64
}

// budgetVars is every configurable bound, in the order they are reported.
var budgetVars = []budgetVar{
	{MaxStepsEnvVar, DefaultMaxSteps, "evaluation steps", func(b *Budgets) *int64 { return &b.MaxSteps }},
	{MaxActionStepsEnvVar, DefaultMaxActionSteps, "action token-flow steps", func(b *Budgets) *int64 { return &b.MaxActionSteps }},
	{MaxStateEventsEnvVar, DefaultMaxStateEvents, "state machine events", func(b *Budgets) *int64 { return &b.MaxStateEvents }},
	{MaxDoStepsEnvVar, DefaultMaxDoSteps, "do activity steps", func(b *Budgets) *int64 { return &b.MaxDoSteps }},
}

// Validate reports every bound that is not positive. A run under a non-positive
// bound could make no progress at all.
func (b Budgets) Validate() error {
	var errs []error
	for _, v := range budgetVars {
		if n := *v.field(&b); n <= 0 {
			errs = append(errs, fmt.Errorf("%s budget must be greater than zero, got %d (%s)", v.counts, n, v.env))
		}
	}
	return errors.Join(errs...)
}

// BudgetsFromEnv returns the bounds the environment asks for: for each variable
// the positive integer it holds, or the default when it is unset or empty. Every
// unusable value is reported, naming its variable and the value, so a typo is
// reported instead of silently leaving the default in place.
func BudgetsFromEnv() (Budgets, error) {
	return budgetsFromLookup(os.Getenv)
}

// budgetsFromLookup is BudgetsFromEnv over an explicit lookup, so the parsing
// rules are testable without the process environment.
func budgetsFromLookup(lookup func(string) string) (Budgets, error) {
	budgets := DefaultBudgets()
	var errs []error
	for _, v := range budgetVars {
		n, err := budgetFromValue(v, lookup(v.env))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		*v.field(&budgets) = n
	}
	if err := errors.Join(errs...); err != nil {
		return Budgets{}, err
	}
	return budgets, nil
}

// budgetFromValue parses one bound's value, defaulting on unset or empty.
func budgetFromValue(v budgetVar, raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return v.def, nil
	}
	n, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not an integer: set it to a positive number of %s (default %d)", v.env, raw, v.counts, v.def)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s=%q must be greater than zero: the budget is what stops a runaway run (default %d)", v.env, raw, v.def)
	}
	return n, nil
}

// Budgets returns the bounds this context runs under.
func (ctx *Context) Budgets() Budgets {
	return Budgets{
		MaxSteps:       ctx.maxSteps,
		MaxActionSteps: ctx.maxActionSteps,
		MaxStateEvents: ctx.maxStateEvents,
		MaxDoSteps:     ctx.maxDoSteps,
	}
}

// SetBudgets replaces the bounds this context runs under, rejecting a set that
// holds a non-positive bound. The evaluation step counter already spent is left
// alone: the budget is a bound on the run, not a reset of it.
func (ctx *Context) SetBudgets(b Budgets) error {
	if err := b.Validate(); err != nil {
		return err
	}
	ctx.maxSteps = b.MaxSteps
	ctx.maxActionSteps = b.MaxActionSteps
	ctx.maxStateEvents = b.MaxStateEvents
	ctx.maxDoSteps = b.MaxDoSteps
	return nil
}
