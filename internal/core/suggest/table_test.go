package suggest_test

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/suggest"
)

// A table swept once answers what the per-name scans answer, so the cheaper
// path is not a different one.
func TestTableAgreesWithScan(t *testing.T) {
	idx := libraryIndex(t)
	table := suggest.NewTable(idx)
	names := suggest.SimpleNames(idx)

	for _, name := range []string{"Integer", "String", "Intger", "Whel", "Zzzznotatype", ""} {
		t.Run(name, func(t *testing.T) {
			if got, want := table.Qualified(name), suggest.Qualified(idx, name); !equal(got, want) {
				t.Errorf("Table.Qualified(%q) = %v, want %v", name, got, want)
			}
			if name == "" {
				return
			}
			if got, want := neighbourNames(table.Neighbours(name)), neighbourNames(suggest.Neighbours(name, names)); !equal(got, want) {
				t.Errorf("Table.Neighbours(%q) = %v, want %v", name, got, want)
			}
		})
	}
}

// A table over no index suggests nothing rather than panicking.
func TestTableWithoutIndex(t *testing.T) {
	table := suggest.NewTable(nil)
	if got := table.Qualified("Integer"); len(got) > 0 {
		t.Errorf("Table.Qualified over no index = %v, want none", got)
	}
	if got := table.Neighbours("Integer"); len(got) > 0 {
		t.Errorf("Table.Neighbours over no index = %v, want none", got)
	}
}

// neighbourNames reads the names off neighbours, in the order they came.
func neighbourNames(hits []suggest.Neighbour) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.Name)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
