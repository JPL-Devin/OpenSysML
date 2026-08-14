package repl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// dropReport records what one submission did to a declaration already in the
// buffer: it either merged into it (keeping the members the new body does not
// redeclare) or replaced it, in which case everything the superseded snippet
// declared and this submission does not is gone.
type dropReport struct {
	merged bool
	decl   string   // the declaration this submission re-declared, e.g. "package P"
	lost   []string // members no longer declared, e.g. "part def A"
}

// notice renders the report as the line the user is told about it, so no
// declaration ever disappears silently.
func (d dropReport) notice() string {
	if d.decl == "" {
		return ""
	}
	switch {
	case d.merged && len(d.lost) == 0:
		return fmt.Sprintf("note: added to the existing %s (its other members are kept)", d.decl)
	case d.merged:
		return fmt.Sprintf("note: added to the existing %s, replacing %s", d.decl, strings.Join(d.lost, ", "))
	case len(d.lost) == 0:
		return fmt.Sprintf("note: replaced %s", d.decl)
	default:
		return fmt.Sprintf("note: replaced %s (%s no longer declared)", d.decl, strings.Join(d.lost, ", "))
	}
}

// dropNotices renders the reports of one submission, skipping the ones with
// nothing to say.
func dropNotices(drops []dropReport) []string {
	var out []string
	for _, d := range drops {
		if note := d.notice(); note != "" {
			out = append(out, note)
		}
	}
	return out
}

// replacedReport describes a snippet a submission supersedes: what it
// re-declared, and what went with it because the whole snippet was replaced —
// both the snippet's other top-level declarations and, for a re-declared
// namespace, the members its new body does not declare again.
func replacedReport(sn snippet, redeclared map[string]bool, newSrc string, newTop map[string]ast.Node) dropReport {
	root := parser.New(source.New(docName, []byte(sn.src))).ParseFile()
	rep := dropReport{}
	if root == nil {
		return rep
	}
	for _, m := range root.Members {
		name := memberName(m)
		if name == "" {
			continue
		}
		desc := renderMember(m)
		if desc == "" {
			desc = name
		}
		if redeclared[name] {
			if rep.decl == "" {
				rep.decl = desc
			}
			rep.lost = append(rep.lost, lostMembers(sn.src, m, newSrc, newTop[name])...)
			continue
		}
		rep.lost = append(rep.lost, desc)
	}
	return rep
}

// lostMembers lists the members of a re-declared namespace that its new body
// leaves undeclared, so replacing a body names what it took with it. A member
// re-declared as a namespace on both sides is descended into: only what neither
// body declares is lost.
func lostMembers(oldSrc string, om ast.Node, newSrc string, nm ast.Node) []string {
	if nm == nil {
		return nil
	}
	oldMembers, ok := bodyMembers(om)
	if !ok {
		return nil
	}
	// A new declaration with no body of its own declares none of the members,
	// so all of them are lost.
	newMembers, _ := bodyMembers(nm)
	var lost []string
	for _, m := range oldMembers {
		name := memberName(m)
		if name == "" {
			continue
		}
		_, kept := findNamed(newMembers, name)
		if kept == nil {
			desc := renderMember(m)
			if desc == "" {
				desc = name
			}
			lost = append(lost, desc)
			continue
		}
		lost = append(lost, lostMembers(oldSrc, m, newSrc, kept)...)
	}
	return lost
}

// instancesDroppedNotice reports the instances a submission invalidated. A new
// document is a new AST, so the objects materialized from the previous one — and
// the IDs they were given — do not carry over.
func instancesDroppedNotice(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("note: %s dropped because the declarations changed; re-run %%instantiate", countOf(n, "instance was", "instances were"))
}

// instancesGoneNote explains an empty instance list that was not always empty,
// which %instances would otherwise report the same way as a fresh session.
func instancesGoneNote(n, version int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("(no instances created; %s dropped when the declarations changed at submission %d — re-run %%instantiate)",
		countOf(n, "instance was", "instances were"), version)
}

func countOf(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// endedSession is why a debugging session is no longer there: the submission
// that invalidated it, so the next command can say what happened instead of only
// that nothing is active.
type endedSession struct {
	kind     string // "action" or "state machine"
	name     string // the behavior that was being debugged
	rootName string // the declaration whose resubmission ended it
	version  int    // the submission that did it
}

// reason explains an ended debugging session in the past tense, for a command
// that found none active.
func (e *endedSession) reason() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("the %s session for %q ended when %s was redeclared at submission %d",
		e.kind, e.name, e.rootName, e.version)
}

// noSessionMsg is what a debugging command reports when nothing is active: why
// the session it would have used is gone, when that is known, and how to start
// one either way.
func noSessionMsg(what string, ended *endedSession, start string) string {
	return "error: " + noSessionText(what, ended, start)
}

// noSessionText is the same message as an error's text, for a command that
// reports its failure as an error rather than as a line.
func noSessionText(what string, ended *endedSession, start string) string {
	msg := "no active " + what
	if reason := ended.reason(); reason != "" {
		msg += ": " + reason
	}
	if start == "" {
		return msg
	}
	return msg + fmt.Sprintf(" (use %s <name> first)", start)
}

// noActionSessionMsg reports that %step and friends have no action executor to
// drive, naming the submission that ended the last one.
func (s *Session) noActionSessionMsg() string {
	return noSessionMsg("action session", s.endedAction, "%action")
}

// noActionSessionErr is noActionSessionMsg as an error.
func (s *Session) noActionSessionErr() error {
	return errors.New(noSessionText("action session", s.endedAction, "%action"))
}

// noStateSessionErr is noStateSessionMsg as an error.
func (s *Session) noStateSessionErr() error {
	return errors.New(noSessionText("state machine session", s.endedState, "%state"))
}

// noStateSessionMsg is the same for the state machine debugger.
func (s *Session) noStateSessionMsg() string {
	return noSessionMsg("state machine session", s.endedState, "%state")
}

// noDebugSessionMsg answers a command that would drive either debugger,
// explaining whichever session ended most recently.
func (s *Session) noDebugSessionMsg() string {
	ended := s.endedAction
	if s.endedState != nil && (ended == nil || s.endedState.version >= ended.version) {
		ended = s.endedState
	}
	return noSessionMsg("debugging session", ended, "")
}
