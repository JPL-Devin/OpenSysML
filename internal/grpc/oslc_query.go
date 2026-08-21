package grpc

import (
	corequery "github.com/Open-MBEE/OpenSysML/internal/core/query"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type coreQueryModel struct {
	eval   *queryEval
	cached *CachedModel
}

func (m *coreQueryModel) Candidates(scope []string) ([]*symbols.Symbol, error) {
	return m.eval.candidates(m.cached, scope)
}

func (m *coreQueryModel) Value(sym *symbols.Symbol, property string) ([]string, bool) {
	value, ok := m.eval.value(sym, property)
	if !ok {
		return nil, false
	}
	return []string{value}, true
}

func (m *coreQueryModel) Identity(sym *symbols.Symbol) string {
	return m.eval.sc.Index.GetFQN(sym)
}

func (m *coreQueryModel) Type(sym *symbols.Symbol) string {
	return corequery.MetamodelTypeNameOf(sym)
}

func queryStatus(err error) error {
	if err == nil {
		return nil
	}
	if qe, ok := err.(*QueryError); ok {
		return qe.GRPCStatus().Err()
	}
	if qe, ok := err.(*corequery.Error); ok {
		return status.Error(codes.InvalidArgument, qe.Message)
	}
	return status.Error(codes.InvalidArgument, err.Error())
}
