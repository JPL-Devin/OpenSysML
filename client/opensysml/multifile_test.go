package opensysml_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const (
	librarySource = "package Lib {\n\tpart def Engine {\n\t\tattribute power = 150;\n\t}\n}\n"
	topSource     = "package Top {\n\tprivate import Lib::*;\n\tpart def Car {\n\t\tpart motor : Engine;\n\t}\n}\n"
)

// writeModelFiles writes the library and the file importing it, and returns
// their paths in that order.
func writeModelFiles(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, 0, 2)
	for _, document := range []struct{ name, content string }{
		{"lib.sysml", librarySource},
		{"top.sysml", topSource},
	} {
		path := filepath.Join(dir, document.name)
		if err := os.WriteFile(path, []byte(document.content), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
		paths = append(paths, path)
	}
	return paths
}

func TestOneDocumentDoesNotSeeWhatAnotherDeclares(t *testing.T) {
	client := newClient(t)
	model, err := client.ParseSource(context.Background(), topSource)
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	if len(model.Diagnostics) == 0 {
		t.Error("a source importing another parsed clean on its own; the one-document scope no longer holds")
	}
}

func TestParseFilesResolvesAnImportBetweenFiles(t *testing.T) {
	paths := writeModelFiles(t)
	bothImplementations(t, func(t *testing.T, client opensysml.Client) {
		model, err := client.ParseFiles(context.Background(), paths)
		if err != nil {
			t.Fatalf("ParseFiles: %v", err)
		}
		if len(model.Diagnostics) != 0 {
			t.Errorf("diagnostics from files that resolve each other: %v", model.Diagnostics)
		}
		if len(model.Roots) != len(paths) {
			t.Errorf("Roots = %d, want one per file (%d)", len(model.Roots), len(paths))
		}
		// The model is one model: a symbol of either file is looked up, and the
		// part typed by the other file's def instantiates.
		for _, id := range []string{"Lib::Engine", "Top::Car"} {
			if _, err := client.LookupSymbol(context.Background(), model, id); err != nil {
				t.Errorf("LookupSymbol(%s): %v", id, err)
			}
		}
		instantiation, err := client.Instantiate(context.Background(), model, "Top::Car")
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		if instantiation.Root == nil {
			t.Error("Instantiate answered no root instance")
		}
		if _, err := client.Evaluate(context.Background(), model, "power",
			opensysml.WithContextSymbol("Lib::Engine")); err != nil {
			t.Errorf("Evaluate: %v", err)
		}
	})
}

func TestParseDocumentsTakesFilesAndInlineSourcesTogether(t *testing.T) {
	paths := writeModelFiles(t)
	bothImplementations(t, func(t *testing.T, client opensysml.Client) {
		model, err := client.ParseDocuments(context.Background(), []opensysml.Document{
			opensysml.File(paths[0]),
			opensysml.Source("top.sysml", topSource),
		})
		if err != nil {
			t.Fatalf("ParseDocuments: %v", err)
		}
		if len(model.Diagnostics) != 0 {
			t.Errorf("diagnostics from a file and a source that resolve each other: %v", model.Diagnostics)
		}
		if _, err := client.LookupSymbol(context.Background(), model, "Top::Car"); err != nil {
			t.Errorf("LookupSymbol: %v", err)
		}
	})
}

func TestADiagnosticNamesTheDocumentItCameFrom(t *testing.T) {
	bothImplementations(t, func(t *testing.T, client opensysml.Client) {
		model, err := client.ParseDocuments(context.Background(), []opensysml.Document{
			opensysml.Source("clean.sysml", librarySource),
			opensysml.Source("broken.sysml", "package Broken {\n\tpart def Wheel {\n\t\tpart hub : Missing;\n\t}\n}\n"),
		})
		if err != nil {
			t.Fatalf("ParseDocuments: %v", err)
		}
		if len(model.Diagnostics) == 0 {
			t.Fatal("a document naming an undeclared type produced no diagnostic")
		}
		for _, diag := range model.Diagnostics {
			if diag.Span == nil {
				continue
			}
			if diag.Span.File != "broken.sysml" {
				t.Errorf("diagnostic located in %q, want broken.sysml: %s", diag.Span.File, diag.Message)
			}
		}
	})
}

func TestParseDocumentsRefusesADocumentSetItCannotParse(t *testing.T) {
	bothImplementations(t, func(t *testing.T, client opensysml.Client) {
		if _, err := client.ParseDocuments(context.Background(), nil); !errors.Is(err, opensysml.CodeInvalidArgument) {
			t.Errorf("no documents: err = %v, want CodeInvalidArgument", err)
		}
		duplicates := []opensysml.Document{
			opensysml.Source("same.sysml", librarySource),
			opensysml.Source("same.sysml", topSource),
		}
		if _, err := client.ParseDocuments(context.Background(), duplicates); !errors.Is(err, opensysml.CodeInvalidArgument) {
			t.Errorf("two documents of one name: err = %v, want CodeInvalidArgument", err)
		}
	})
}

func TestParseFilesReportsAMissingFile(t *testing.T) {
	paths := writeModelFiles(t)
	bothImplementations(t, func(t *testing.T, client opensysml.Client) {
		absent := filepath.Join(filepath.Dir(paths[0]), "absent.sysml")
		_, err := client.ParseFiles(context.Background(), []string{paths[0], absent})
		if !errors.Is(err, opensysml.CodeNotFound) {
			t.Errorf("err = %v, want CodeNotFound", err)
		}
	})
}

func TestTheSameDocumentsParseToTheSameModel(t *testing.T) {
	paths := writeModelFiles(t)
	client := newClient(t)
	first, err := client.ParseFiles(context.Background(), paths)
	if err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}
	second, err := client.ParseFiles(context.Background(), paths)
	if err != nil {
		t.Fatalf("second ParseFiles: %v", err)
	}
	if first.Hash != second.Hash {
		t.Errorf("hashes differ for the same documents: %s and %s", first.Hash, second.Hash)
	}
	// Order is part of what was parsed, since the first document is the model's
	// primary one.
	reversed, err := client.ParseFiles(context.Background(), []string{paths[1], paths[0]})
	if err != nil {
		t.Fatalf("reversed ParseFiles: %v", err)
	}
	if reversed.Hash == first.Hash {
		t.Error("the same files in another order answered one model hash")
	}
}

func TestParseSourcesIsReportedAsACapability(t *testing.T) {
	client := newClient(t)
	info, err := client.ServerInfo(context.Background())
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	if !info.Has(opensysml.CapabilityParseSources) {
		t.Errorf("capabilities %v do not name %s", info.Capabilities, opensysml.CapabilityParseSources)
	}
}
