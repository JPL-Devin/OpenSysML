package lower

import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// Test trigger classification in lowerTransitionMember
func TestTriggerClassification_SignalTrigger(t *testing.T) {
	src := `
		package test {
			state M {
				initial start;
				state waiting;
				final done;
				
				start then waiting;
				transition waiting to done when powerOn;
			}
		}
	`
	
	file := source.New("test.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	
	// Find state usage
	var stateUsage *ast.Usage
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			if pkg, ok := membership.Member.(*ast.Package); ok {
				for _, pkgMember := range pkg.Members {
					if pkgMembership, ok := pkgMember.(*ast.Membership); ok {
						if usage, ok := pkgMembership.Member.(*ast.Usage); ok && usage.Kind == ast.UsageState {
							stateUsage = usage
							break
						}
					}
				}
			}
		}
	}
	
	if stateUsage == nil {
		t.Fatal("no state usage found")
	}
	
	graph, err := ToStateGraph(stateUsage)
	if err != nil {
		t.Fatalf("ToStateGraph failed: %v", err)
	}
	
	// Find transition from waiting to done
	var transition *Transition
	var transMember *ast.TransitionMember
	// First find the raw TransitionMember to see what trigger type the parser created
	for _, stateMember := range stateUsage.Members {
		if memb, ok := stateMember.(*ast.Membership); ok {
			if trans, ok := memb.Member.(*ast.TransitionMember); ok {
				if trans.Source != nil && len(trans.Source.Parts) > 0 && trans.Source.Parts[0].Text == "waiting" {
					transMember = trans
					break
				}
			}
		}
	}
	
	if transMember != nil {
		t.Logf("Raw trigger type before classification: %T", transMember.Trigger)
		if qname, ok := transMember.Trigger.(*ast.QualifiedName); ok {
			t.Logf("QualifiedName parts: %v", qname.Parts)
		}
	}
	
	for _, trans := range graph.Transitions {
		for _, tr := range trans {
			if stateNode, ok := tr.Source.(*ast.StateNode); ok && stateNode.Name == "waiting" {
				if targetNode, ok := tr.Target.(*ast.StateNode); ok && targetNode.Name == "done" {
					transition = tr
					break
				}
			}
		}
	}
	
	if transition == nil {
		t.Fatal("transition waiting->done not found")
	}
	
	// Trigger should be classified as AcceptEvent
	acceptEvent, ok := transition.Trigger.(*ast.AcceptEvent)
	if !ok {
		t.Fatalf("expected AcceptEvent, got %T", transition.Trigger)
	}
	
	// SignalType should be QualifiedName with "powerOn"
	if acceptEvent.SignalType == nil {
		t.Fatal("AcceptEvent has nil SignalType")
	}
	
	if len(acceptEvent.SignalType.Parts) != 1 || acceptEvent.SignalType.Parts[0].Text != "powerOn" {
		t.Errorf("expected signal 'powerOn', got %v", acceptEvent.SignalType)
	}
}

func TestTriggerClassification_ChangeEvent(t *testing.T) {
	src := `
		package test {
			state M {
				initial start;
				state waiting;
				final done;
				
				start then waiting;
				transition waiting to done when x > 5;
			}
		}
	`
	
	file := source.New("test.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	
	// Find state usage
	var stateUsage *ast.Usage
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			if pkg, ok := membership.Member.(*ast.Package); ok {
				for _, pkgMember := range pkg.Members {
					if pkgMembership, ok := pkgMember.(*ast.Membership); ok {
						if usage, ok := pkgMembership.Member.(*ast.Usage); ok && usage.Kind == ast.UsageState {
							stateUsage = usage
							break
						}
					}
				}
			}
		}
	}
	
	if stateUsage == nil {
		t.Fatal("no state usage found")
	}
	
	graph, err := ToStateGraph(stateUsage)
	if err != nil {
		t.Fatalf("ToStateGraph failed: %v", err)
	}
	
	// Find transition
	var transition *Transition
	for _, trans := range graph.Transitions {
		for _, t := range trans {
			if stateNode, ok := t.Source.(*ast.StateNode); ok && stateNode.Name == "waiting" {
				transition = t
				break
			}
		}
	}
	
	if transition == nil {
		t.Fatal("transition not found")
	}
	
	// Trigger should be classified as ChangeEvent
	changeEvent, ok := transition.Trigger.(*ast.ChangeEvent)
	if !ok {
		t.Fatalf("expected ChangeEvent, got %T", transition.Trigger)
	}
	
	// Condition should be the expression
	if changeEvent.Condition == nil {
		t.Fatal("ChangeEvent has nil Condition")
	}
}

func TestTriggerClassification_NilCompletion(t *testing.T) {
	src := `
		package test {
			state M {
				initial start;
				state waiting;
				final done;
				
				start then waiting;
				transition waiting to done;
			}
		}
	`
	
	file := source.New("test.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	
	// Find state usage
	var stateUsage *ast.Usage
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			if pkg, ok := membership.Member.(*ast.Package); ok {
				for _, pkgMember := range pkg.Members {
					if pkgMembership, ok := pkgMember.(*ast.Membership); ok {
						if usage, ok := pkgMembership.Member.(*ast.Usage); ok && usage.Kind == ast.UsageState {
							stateUsage = usage
							break
						}
					}
				}
			}
		}
	}
	
	if stateUsage == nil {
		t.Fatal("no state usage found")
	}
	
	graph, err := ToStateGraph(stateUsage)
	if err != nil {
		t.Fatalf("ToStateGraph failed: %v", err)
	}
	
	// Find transition
	var transition *Transition
	for _, trans := range graph.Transitions {
		for _, t := range trans {
			if stateNode, ok := t.Source.(*ast.StateNode); ok && stateNode.Name == "waiting" {
				transition = t
				break
			}
		}
	}
	
	if transition == nil {
		t.Fatal("transition not found")
	}
	
	// Trigger should be nil (completion transition)
	if transition.Trigger != nil {
		t.Fatalf("expected nil trigger, got %T", transition.Trigger)
	}
}

func TestTriggerClassification_AlreadyTyped(t *testing.T) {
	// Test that already-typed triggers pass through unchanged
	timeEvent := &ast.TimeEvent{Duration: &ast.LiteralInteger{Value: "5"}}
	result := classifyTrigger(timeEvent)
	
	if result != timeEvent {
		t.Error("TimeEvent should pass through unchanged")
	}
	
	changeEvent := &ast.ChangeEvent{Condition: &ast.LiteralBool{Value: true}}
	result = classifyTrigger(changeEvent)
	
	if result != changeEvent {
		t.Error("ChangeEvent should pass through unchanged")
	}
	
	acceptEvent := &ast.AcceptEvent{SignalType: &ast.QualifiedName{}}
	result = classifyTrigger(acceptEvent)
	
	if result != acceptEvent {
		t.Error("AcceptEvent should pass through unchanged")
	}
	
	callEvent := &ast.CallEvent{Operation: &ast.QualifiedName{}}
	result = classifyTrigger(callEvent)
	
	if result != callEvent {
		t.Error("CallEvent should pass through unchanged")
	}
}
