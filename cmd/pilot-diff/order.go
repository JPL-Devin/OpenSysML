package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// orderByImports sorts files so that every file follows the files declaring the
// root namespaces it imports. Files the parse cannot relate keep their original
// order, as do the members of an import cycle.
//
// This is only needed for the pilot side: it processes inputs one after another
// like notebook cells, so `private import 'Part Definition Example'::*` fails
// unless that file was fed in first. Our own workspace opens the batch before
// diagnosing it and needs no ordering.
func orderByImports(repo, dir string, files []string) []string {
	declaredBy := make(map[string]string, len(files))
	declaredByFile := make(map[string]map[string]bool, len(files))
	imports := make(map[string][]string, len(files))
	for _, rel := range files {
		// #nosec G304 -- the corpus root to compare is named on the command line.
		content, err := os.ReadFile(filepath.Join(repo, dir, rel))
		if err != nil {
			// The file is read again on both analysis paths, which report the
			// failure properly; ordering just cannot use it.
			fmt.Fprintf(os.Stderr, "ordering: read %s: %v\n", rel, err)
			continue
		}
		root := parser.New(source.New(rel, content)).ParseFile()
		declared := make(map[string]bool)
		for _, name := range declaredNames(root) {
			declared[name] = true
			if _, ok := declaredBy[name]; !ok {
				declaredBy[name] = rel
			}
		}
		declaredByFile[rel] = declared
		imports[rel] = importedNames(root)
	}

	// Kahn's algorithm over "declaring file -> importing file", taking ready
	// files in the input order so the result is stable.
	dependencies := make(map[string]map[string]bool, len(files))
	dependents := make(map[string][]string, len(files))
	for _, rel := range files {
		dependencies[rel] = make(map[string]bool)
	}
	for _, rel := range files {
		for _, name := range imports[rel] {
			if declaredByFile[rel][name] {
				continue
			}
			provider, ok := declaredByByPrefix(name, declaredBy)
			if !ok || provider == rel || dependencies[rel][provider] {
				continue
			}
			dependencies[rel][provider] = true
			dependents[provider] = append(dependents[provider], rel)
		}
	}

	ordered := make([]string, 0, len(files))
	remaining := make(map[string]bool, len(files))
	for _, rel := range files {
		remaining[rel] = true
	}
	for len(ordered) < len(files) {
		progressed := false
		for _, rel := range files {
			if !remaining[rel] || len(dependencies[rel]) > 0 {
				continue
			}
			ordered = append(ordered, rel)
			delete(remaining, rel)
			for _, dependent := range dependents[rel] {
				delete(dependencies[dependent], rel)
			}
			progressed = true
		}
		if progressed {
			continue
		}
		// A cycle (or a self-referential import chain): release the first
		// remaining file in input order and continue.
		for _, rel := range files {
			if remaining[rel] {
				ordered = append(ordered, rel)
				delete(remaining, rel)
				for _, dependent := range dependents[rel] {
					delete(dependencies[dependent], rel)
				}
				break
			}
		}
	}
	return ordered
}

// declaredNames returns the namespace names the file declares, including nested
// packages and namespaces.
func declaredNames(root *ast.RootNamespace) []string {
	seen := make(map[string]bool)
	var walk func([]ast.Node, []string)
	walk = func(members []ast.Node, prefixes []string) {
		if len(prefixes) == 0 {
			prefixes = []string{""}
		}
		for _, member := range members {
			if membership, ok := member.(*ast.Membership); ok {
				member = membership.Member
			}
			switch node := member.(type) {
			case *ast.Package:
				walk(node.Members, addDeclaredNames(seen, prefixes, node.Ident))
			case *ast.Namespace:
				walk(node.Members, addDeclaredNames(seen, prefixes, node.Ident))
			}
		}
	}
	walk(root.Members, []string{""})
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func addDeclaredNames(seen map[string]bool, prefixes []string, ident ast.Identification) []string {
	var next []string
	add := func(prefix, name string) {
		name = unquote(name)
		if name == "" {
			return
		}
		full := name
		if prefix != "" {
			full = prefix + "::" + name
		}
		if !seen[full] {
			seen[full] = true
		}
		next = append(next, full)
	}
	if len(prefixes) == 0 {
		prefixes = []string{""}
	}
	for _, prefix := range prefixes {
		add(prefix, ident.Name)
		add(prefix, ident.ShortName)
	}
	return next
}

// importedNames returns the namespace names referenced by the file's imports.
func importedNames(root *ast.RootNamespace) []string {
	seen := make(map[string]bool)
	walkImports(root, func(imp *ast.Import) {
		if imp.Imported == nil || len(imp.Imported.Parts) == 0 {
			return
		}
		if name := qualifiedName(imp.Imported); name != "" {
			seen[name] = true
		}
	})
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func declaredByByPrefix(name string, declaredBy map[string]string) (string, bool) {
	for {
		if provider, ok := declaredBy[name]; ok {
			return provider, true
		}
		i := strings.LastIndex(name, "::")
		if i < 0 {
			return "", false
		}
		name = name[:i]
	}
}

func qualifiedName(qn *ast.QualifiedName) string {
	if qn == nil || len(qn.Parts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(qn.Parts))
	for _, part := range qn.Parts {
		if text := unquote(part.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "::")
}

// walkImports visits every Import in the tree, including imports nested in
// definition and usage bodies.
func walkImports(node ast.Node, visit func(*ast.Import)) {
	var members []ast.Node
	switch n := node.(type) {
	case *ast.RootNamespace:
		members = n.Members
	case *ast.Package:
		members = n.Members
	case *ast.Namespace:
		members = n.Members
	case *ast.Definition:
		members = n.Members
	case *ast.Usage:
		members = n.Members
	case *ast.Membership:
		members = []ast.Node{n.Member}
	case *ast.Alias:
		members = n.Body
	case *ast.Import:
		visit(n)
		members = n.Body
	}
	for _, member := range members {
		if member != nil {
			walkImports(member, visit)
		}
	}
}

// unquote strips the quotes of a restricted name (`'Part Definitions'`).
func unquote(name string) string {
	return strings.Trim(name, "'")
}
