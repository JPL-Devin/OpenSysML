package runtime

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DefaultMaxSteps is the evaluation step budget used when MaxStepsEnvVar names
// no override.
const DefaultMaxSteps int64 = 100000

// MaxStepsEnvVar is the environment variable that overrides DefaultMaxSteps,
// following the SYSML_LIBRARY_PATH convention.
const MaxStepsEnvVar = "SYSML_MAX_STEPS"

// MaxStepsFromEnv returns the evaluation step budget to pass to NewContext: the
// positive integer MaxStepsEnvVar holds, or DefaultMaxSteps when it is unset or
// empty. Any other value is an error naming the variable and the value, so a
// typo is reported instead of silently leaving the default in place.
func MaxStepsFromEnv() (int64, error) {
	return maxStepsFromValue(os.Getenv(MaxStepsEnvVar))
}

// MaxSteps returns the evaluation step budget this context runs under.
func (ctx *Context) MaxSteps() int64 {
	return ctx.maxSteps
}

// maxStepsFromValue is MaxStepsFromEnv over an explicit value, so the parsing
// rules are testable without the process environment.
func maxStepsFromValue(raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return DefaultMaxSteps, nil
	}
	n, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not an integer: set it to a positive number of evaluation steps (default %d)", MaxStepsEnvVar, raw, DefaultMaxSteps)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s=%q must be greater than zero: the step budget is what stops a runaway evaluation (default %d)", MaxStepsEnvVar, raw, DefaultMaxSteps)
	}
	return n, nil
}
