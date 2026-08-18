package grpc

import (
	"context"
	"errors"
	"fmt"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Verdict kinds, as reported in Verdict.kind.
const (
	verdictConstraint  = "constraint"
	verdictRequirement = "requirement"
	verdictSatisfy     = "satisfy"
)

// failureReason classifies a failure for the wire, so a client acts on the kind
// of failure rather than on the message text.
func failureReason(err error) pb.FailureReason {
	switch {
	case err == nil:
		return pb.FailureReason_FAILURE_REASON_UNSPECIFIED
	case errors.Is(err, runtime.ErrAmbiguousSubject):
		return pb.FailureReason_FAILURE_REASON_AMBIGUOUS_SUBJECT
	case errors.Is(err, runtime.ErrNotAConstraint),
		errors.Is(err, runtime.ErrNotARequirement),
		errors.Is(err, runtime.ErrNotASatisfaction),
		errors.Is(err, runtime.ErrNotACalc):
		return pb.FailureReason_FAILURE_REASON_WRONG_KIND
	default:
		return pb.FailureReason_FAILURE_REASON_EVALUATION
	}
}

// verifyContext is everything a verification RPC needs from a cached model: the
// runtime it evaluates in and the index its names resolve against.
type verifyContext struct {
	cached  *CachedModel
	runtime *runtime.Context
	sem     *semantics.Model
}

// newVerifyContext looks the model up and builds a runtime over it, the same way
// every other runtime RPC in this service does.
func (s *Service) newVerifyContext(modelHash string) (*verifyContext, error) {
	cached, ok := s.cache.Get(modelHash)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "model not found: %s", modelHash)
	}
	resolver := resolve.New(cached.Index)
	semModel := semantics.NewModel(resolver)
	return &verifyContext{cached: cached, runtime: s.newRuntime(semModel, resolver), sem: semModel}, nil
}

// lookup resolves an FQN to the symbol it names.
func (v *verifyContext) lookup(symbolID string) (*symbols.Symbol, error) {
	syms := v.cached.Index.LookupQualified(symbolID)
	if len(syms) == 0 {
		return nil, fmt.Errorf("symbol not found: %s", symbolID)
	}
	return syms[0], nil
}

// declaringScope is the scope an element's conditions were written in, which is
// what their names resolve against; the document root is the fallback for a
// symbol carrying no declaring scope.
func (v *verifyContext) declaringScope(sym *symbols.Symbol) *symbols.Scope {
	if sym != nil && sym.OwnerScope != nil {
		return sym.OwnerScope
	}
	return v.cached.Index.DocumentRoot(v.cached.Source.Name())
}

// subject instantiates the part/usage a request named, so a verdict can be about
// concrete values. An empty name is no subject, which is not an error: the
// verdict is then about declared defaults.
func (v *verifyContext) subject(symbolID string) (*runtime.Instance, error) {
	if symbolID == "" {
		return nil, nil
	}
	sym, err := v.lookup(symbolID)
	if err != nil {
		return nil, err
	}
	inst, err := v.runtime.Instantiate(sym)
	if err != nil {
		return nil, fmt.Errorf("instantiation of subject %s failed: %w", symbolID, err)
	}
	return inst, nil
}

// verdict builds the answer to one verification. A condition that evaluated to
// false is the model's answer, reported as holds=false with the condition named;
// any other error is a failure to evaluate, reported in Verdict.error.
func (v *verifyContext) verdict(kind string, sym *symbols.Symbol, element string, inst *runtime.Instance, holds bool, err error) *pb.Verdict {
	out := &pb.Verdict{
		Kind:    kind,
		Element: element,
		Holds:   holds && err == nil,
	}
	if sym != nil {
		out.ElementId = v.cached.Index.GetFQN(sym)
		if out.Element == "" {
			out.Element = out.ElementId
		}
	}
	if inst != nil {
		out.InstanceId = inst.ID
		out.InstanceTypeId = v.cached.Index.GetFQN(inst.Type)
	}

	var violation *runtime.ViolationError
	switch {
	case err == nil:
		// Nothing to explain: holds is the model's answer either way.
	case errors.As(err, &violation):
		out.Condition = violation.Condition
	case errors.Is(err, runtime.ErrViolated):
		// A verdict of false the runtime did not attribute to one condition.
	default:
		out.Error = err.Error()
		out.FailureReason = failureReason(err)
	}
	return out
}

// instanceGraph serializes the object a verdict is about, so a client can read
// the feature values behind a failure without a follow-up call.
func (v *verifyContext) instanceGraph(inst *runtime.Instance) []*pb.Instance {
	if inst == nil {
		return nil
	}
	_, all := InstanceGraphToProto(v.runtime, inst, v.cached.Index)
	return all
}

// VerifyConstraint evaluates a constraint definition or usage, as the REPL's
// %constraint does.
func (s *Service) VerifyConstraint(ctx context.Context, req *pb.VerifyConstraintRequest) (*pb.VerifyConstraintResponse, error) {
	v, err := s.newVerifyContext(req.ModelHash)
	if err != nil {
		return nil, err
	}
	sym, err := v.lookup(req.SymbolId)
	if err != nil {
		return &pb.VerifyConstraintResponse{Error: err.Error()}, nil
	}
	inst, err := v.subject(req.SubjectSymbolId)
	if err != nil {
		return &pb.VerifyConstraintResponse{Error: err.Error()}, nil
	}

	result, evalErr := v.runtime.CheckConstraintOn(sym, v.declaringScope(sym), inst)
	subject := subjectOf(result, inst)
	return &pb.VerifyConstraintResponse{
		Verdict:   v.verdict(verdictConstraint, sym, "", subject, result.Holds, evalErr),
		Instances: v.instanceGraph(subject),
	}, nil
}

// VerifyRequirement evaluates a requirement definition or usage, as the REPL's
// %requirement does.
func (s *Service) VerifyRequirement(ctx context.Context, req *pb.VerifyRequirementRequest) (*pb.VerifyRequirementResponse, error) {
	v, err := s.newVerifyContext(req.ModelHash)
	if err != nil {
		return nil, err
	}
	sym, err := v.lookup(req.SymbolId)
	if err != nil {
		return &pb.VerifyRequirementResponse{Error: err.Error()}, nil
	}
	inst, err := v.subject(req.SubjectSymbolId)
	if err != nil {
		return &pb.VerifyRequirementResponse{Error: err.Error()}, nil
	}

	result, evalErr := v.runtime.CheckRequirementOn(sym, v.declaringScope(sym), inst)
	subject := subjectOf(result, inst)
	return &pb.VerifyRequirementResponse{
		Verdict:   v.verdict(verdictRequirement, sym, "", subject, result.Holds, evalErr),
		Instances: v.instanceGraph(subject),
	}, nil
}

// VerifySatisfaction evaluates the satisfaction assertions a model states, as the
// REPL's %satisfy does: every one, or, given a symbol, the ones that element
// states — or that element itself when it is a named satisfy assertion.
func (s *Service) VerifySatisfaction(ctx context.Context, req *pb.VerifySatisfactionRequest) (*pb.VerifySatisfactionResponse, error) {
	v, err := s.newVerifyContext(req.ModelHash)
	if err != nil {
		return nil, err
	}

	scope := v.cached.Index.DocumentRoot(v.cached.Source.Name())
	if req.SymbolId != "" {
		sym, lerr := v.lookup(req.SymbolId)
		if lerr != nil {
			return &pb.VerifySatisfactionResponse{Error: lerr.Error()}, nil
		}
		// A named `satisfy requirement r by p` is itself one assertion.
		if a, aerr := v.runtime.SatisfyAssertionOf(sym); aerr == nil {
			verdict, instances := v.satisfyVerdict(a)
			return &pb.VerifySatisfactionResponse{
				Verdicts:  []*pb.Verdict{verdict},
				Instances: instances,
			}, nil
		}
		// A symbol that is neither an assertion nor a scope stating any is the
		// wrong kind of thing to ask about, not an undecided answer.
		if sym.Scope == nil {
			return &pb.VerifySatisfactionResponse{
				Error:         fmt.Sprintf("%s states no satisfaction assertion", req.SymbolId),
				FailureReason: pb.FailureReason_FAILURE_REASON_WRONG_KIND,
			}, nil
		}
		scope = sym.Scope
	}

	resp := &pb.VerifySatisfactionResponse{}
	seen := map[int64]bool{}
	for _, a := range v.runtime.SatisfyAssertionsIn(scope) {
		verdict, instances := v.satisfyVerdict(a)
		resp.Verdicts = append(resp.Verdicts, verdict)
		// One graph per response, so two assertions about the same object do
		// not report it twice.
		for _, inst := range instances {
			if !seen[inst.Id] {
				seen[inst.Id] = true
				resp.Instances = append(resp.Instances, inst)
			}
		}
	}
	return resp, nil
}

// satisfyVerdict evaluates one assertion against an object of its subject, built
// for this call so the verdict is about the values that subject holds.
func (v *verifyContext) satisfyVerdict(a *runtime.SatisfyAssertion) (*pb.Verdict, []*pb.Instance) {
	var subject *runtime.Instance
	if a.Subject != nil {
		// Created here rather than inside the evaluation so that the object the
		// verdict is about can be reported with it.
		inst, err := v.runtime.SatisfySubject(a)
		if err != nil {
			// The assertion cannot be evaluated without the object it is about,
			// which is a failure to evaluate rather than a verdict of false.
			return v.verdict(verdictSatisfy, a.Symbol, a.Text(), nil, false, err), nil
		}
		subject = inst
	}
	result, err := v.runtime.CheckSatisfactionOn(a, subject)
	subject = subjectOf(result, subject)
	return v.verdict(verdictSatisfy, a.Symbol, a.Text(), subject, result.Holds, err),
		v.instanceGraph(subject)
}

// subjectOf is the object a verdict is about: the one the runtime evaluated the
// check against, which for a check reached through a nested redefinition is not
// the object supplied. fallback covers a check that never reached evaluation.
func subjectOf(result runtime.CheckResult, fallback *runtime.Instance) *runtime.Instance {
	if result.Subject != nil {
		return result.Subject
	}
	return fallback
}

// EvaluateCalc invokes a calculation, as the REPL's %calc does: a calc usage
// named with no arguments is evaluated from its own members and reports every
// output feature it computes (SysML 7.17); anything else is invoked with the
// arguments given, bound positionally.
func (s *Service) EvaluateCalc(ctx context.Context, req *pb.EvaluateCalcRequest) (*pb.EvaluateCalcResponse, error) {
	v, err := s.newVerifyContext(req.ModelHash)
	if err != nil {
		return nil, err
	}
	sym, err := v.lookup(req.SymbolId)
	if err != nil {
		return &pb.EvaluateCalcResponse{Error: err.Error()}, nil
	}

	if len(req.Arguments) == 0 {
		outputs, handled, cerr := v.calcUsageOutputs(sym)
		if cerr != nil {
			return &pb.EvaluateCalcResponse{
				Error:         cerr.Error(),
				FailureReason: failureReason(cerr),
			}, nil
		}
		if handled {
			return &pb.EvaluateCalcResponse{Outputs: outputs}, nil
		}
	}

	// Converted against the model's index, so a quantity argument keeps the base
	// units it is commensurable with instead of arriving as an unusable value.
	args := make([]runtime.Value, 0, len(req.Arguments))
	for _, arg := range req.Arguments {
		val, cerr := ProtoToValueIn(arg, v.cached.Index, v.sem)
		if cerr != nil {
			return &pb.EvaluateCalcResponse{
				Error:         fmt.Sprintf("calc argument could not be read: %v", cerr),
				FailureReason: failureReason(cerr),
			}, nil
		}
		args = append(args, val)
	}

	result, err := v.runtime.InvokeCalc(sym, args, v.declaringScope(sym))
	if err != nil {
		return &pb.EvaluateCalcResponse{
			Error:         fmt.Sprintf("calc invocation failed: %v", err),
			FailureReason: failureReason(err),
		}, nil
	}
	return &pb.EvaluateCalcResponse{Result: ValueToProtoIn(v.runtime, result, v.cached.Index)}, nil
}

// calcUsageOutputs evaluates a calc usage from its own member values. It reports
// handled=false when sym is not a calc usage, or is one computing no output
// features, so those are invoked as calculations instead.
func (v *verifyContext) calcUsageOutputs(sym *symbols.Symbol) ([]*pb.CalcOutput, bool, error) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageCalc {
		return nil, false, nil
	}
	outputs, err := v.runtime.CalcUsageOutputs(sym, sym.OwnerScope, nil)
	if err != nil {
		return nil, true, fmt.Errorf("calc usage evaluation failed: %w", err)
	}
	if len(outputs) == 0 {
		return nil, false, nil
	}
	pbOutputs := make([]*pb.CalcOutput, 0, len(outputs))
	for _, out := range outputs {
		pbOutputs = append(pbOutputs, &pb.CalcOutput{
			Name:  out.Name,
			Value: ValueToProtoIn(v.runtime, out.Value, v.cached.Index),
		})
	}
	return pbOutputs, true, nil
}
