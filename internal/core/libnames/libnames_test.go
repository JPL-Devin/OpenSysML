package libnames_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libnames"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
)

// Every declaration the table names is one the bundled library declares
// exactly once, under the unqualified name the table files it by.
func TestDeclarationsNameBundledLibraryFunctions(t *testing.T) {
	idx := libs.SharedBase()
	for _, name := range libnames.Names() {
		decls := libnames.Declarations(name)
		if len(decls) == 0 {
			t.Errorf("Declarations(%q) is empty", name)
		}
		for _, fqn := range decls {
			if got := fqn[strings.LastIndex(fqn, "::")+2:]; got != name {
				t.Errorf("Declarations(%q) lists %s, whose local name is %q", name, fqn, got)
			}
			if n := len(idx.LookupQualified(fqn)); n != 1 {
				t.Errorf("Declarations(%q) lists %s, which the library declares %d time(s)", name, fqn, n)
			}
		}
	}
}

// An extension function is declared by the package a bare call must import,
// and is not also an OMG library name the table would answer unimported.
func TestExtensionsAreImportGatedAndDisjoint(t *testing.T) {
	idx := libs.SharedBase()
	for _, name := range libnames.ExtensionNames() {
		pkg, ok := libnames.ExtensionPackage(name)
		if !ok {
			t.Fatalf("ExtensionPackage(%q) not found", name)
		}
		if n := len(idx.LookupQualified(pkg + "::" + name)); n != 1 {
			t.Errorf("%s::%s declared %d time(s), want 1", pkg, name, n)
		}
		if libnames.Declarations(name) != nil {
			t.Errorf("%q is both an extension and an OMG library name", name)
		}
	}
	if _, ok := libnames.ExtensionPackage("sqrt"); ok {
		t.Error("sqrt reported as an extension function")
	}
	if libnames.Declarations("nosuchfunction") != nil {
		t.Error("an undeclared name has declarations")
	}
}
