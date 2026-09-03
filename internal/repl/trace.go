package repl

import "github.com/Open-MBEE/OpenSysML/internal/core/runtime"

// tracePrefix marks a recorded execution step, so a trace is distinguishable
// from a command's own output.
const tracePrefix = "[trace] "

// Tracing reports whether execution steps are being recorded.
func (s *Session) Tracing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.trace != nil
}

// SetTracing turns recording of execution steps on or off. It takes effect at
// once, on the session's runtime context and on a debugging session already
// under way as well as on everything created afterwards.
func (s *Session) SetTracing(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setTracing(on)
}

func (s *Session) setTracing(on bool) {
	switch {
	case on && s.trace == nil:
		s.trace = runtime.NewTraceRecorder()
	case !on:
		s.trace = nil
	}
	if s.rtCtx != nil {
		s.rtCtx.SetTrace(s.trace)
	}
	if s.actionExec != nil {
		s.actionExec.executor.SetTrace(s.trace)
	}
	if s.stateExec != nil {
		s.stateExec.executor.SetTrace(s.trace)
	}
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// drainTrace returns what was recorded since the last command, prefixed, and
// resets the recorder so each command reports only its own steps.
func (s *Session) drainTrace() []string {
	if s.trace == nil {
		return nil
	}
	entries := s.trace.Entries()
	if len(entries) == 0 {
		return nil
	}
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = tracePrefix + e
	}
	s.trace.Clear()
	return out
}
