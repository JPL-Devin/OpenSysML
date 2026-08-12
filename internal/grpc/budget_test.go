package grpc

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/runtime"
)

// TestNewServiceResolvesMaxSteps: the service reads the step budget once at
// construction, defaulting when the variable is unset.
func TestNewServiceResolvesMaxSteps(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv(runtime.MaxStepsEnvVar, "")
		svc, err := NewService(4)
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		if svc.maxSteps != runtime.DefaultMaxSteps {
			t.Errorf("maxSteps = %d, want the default %d", svc.maxSteps, runtime.DefaultMaxSteps)
		}
	})

	t.Run("raised", func(t *testing.T) {
		t.Setenv(runtime.MaxStepsEnvVar, "1234567")
		svc, err := NewService(4)
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		if svc.maxSteps != 1234567 {
			t.Errorf("maxSteps = %d, want 1234567", svc.maxSteps)
		}
	})
}

// TestNewServiceRejectsUnusableMaxSteps: a value that is not a positive integer
// fails service construction with a message naming the variable, rather than
// falling back to the default silently.
func TestNewServiceRejectsUnusableMaxSteps(t *testing.T) {
	for _, value := range []string{"0", "-1", "plenty"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(runtime.MaxStepsEnvVar, value)
			svc, err := NewService(4)
			if err == nil {
				t.Fatalf("NewService accepted %s=%q", runtime.MaxStepsEnvVar, value)
			}
			if svc != nil {
				t.Error("expected no service alongside the error")
			}
			if !strings.Contains(err.Error(), runtime.MaxStepsEnvVar) {
				t.Errorf("error %q does not name %s", err, runtime.MaxStepsEnvVar)
			}
		})
	}
}
