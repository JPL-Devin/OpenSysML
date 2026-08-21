package query

import "testing"

func TestParseOSLCOperatorsAndValues(t *testing.T) {
	q, err := ParseOSLC(`sysml:name="wheel" and sysml:multiplicityLower>=2 and sysml:type in [sysml:PartUsage, "x"]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Where) != 3 || q.Where[1].Operator != GreaterEqual || q.Where[2].Operator != In {
		t.Fatalf("where = %#v", q.Where)
	}
	if q.Where[0].Values[0] != "wheel" {
		t.Fatalf("string value = %#v", q.Where[0].Values)
	}
	if q.Where[2].Values[0] != "PartUsage" {
		t.Fatalf("prefixed value = %#v", q.Where[2].Values)
	}
}

func TestParseOSLCParametersAndPrefixes(t *testing.T) {
	q, err := ParseParameters(`oslc.prefix=ex%3D%3Curn%3Aexample%3A%3E&oslc.where=sysml%3Aname%3D%22x%22&oslc.select=sysml%3Aname%2Crdf%3Atype&oslc.orderBy=-sysml%3Aname`)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Select) != 2 || q.Select[0] != PropertyName || q.Select[1] != PropertyType {
		t.Fatalf("select = %#v", q.Select)
	}
	if len(q.OrderBy) != 1 || !q.OrderBy[0].Desc {
		t.Fatalf("order = %#v", q.OrderBy)
	}
}

func TestParseOSLCRejectsUnsupportedConstructs(t *testing.T) {
	tests := []struct {
		text string
		kind ErrorKind
	}{
		{`sysml:owner{sysml:name="x"}`, ErrUnsupportedScopedTerm},
		{`oslc.select=*`, ErrUnsupportedWildcard},
		{`oslc.orderBy=*`, ErrUnsupportedWildcard},
		{`oslc.where=*=x`, ErrUnsupportedWildcard},
		{`oslc.properties=*`, ErrUnsupportedWildcard},
		{`oslc.searchTerms=x`, ErrUnsupportedSearchTerms},
		{`oslc.searchTerms=`, ErrUnsupportedSearchTerms},
		{`sysml:name="x"@en`, ErrUnsupportedLiteral},
	}
	for _, tt := range tests {
		q, err := ParseOSLC(tt.text)
		if err == nil {
			t.Errorf("%q unexpectedly parsed as %#v", tt.text, q)
			continue
		}
		got, ok := err.(*Error)
		if !ok || got.Kind != tt.kind {
			t.Errorf("%q error = %#v, want kind %d", tt.text, err, tt.kind)
		}
	}
}

func TestParseOSLCAllowsInfinityForOrderedMultiplicity(t *testing.T) {
	for _, text := range []string{
		`sysml:multiplicityUpper>=*`,
		`sysml:multiplicityUpper=*`,
		`sysml:multiplicityUpper in [*, 1]`,
		`sysml:name=*`,
	} {
		q, err := ParseOSLC(text)
		if err != nil {
			t.Fatalf("ParseOSLC(%q): %v", text, err)
		}
		if len(q.Where) != 1 || q.Where[0].Values[0] != "*" {
			t.Fatalf("ParseOSLC(%q) = %#v", text, q.Where)
		}
	}
}

func TestOSLCPropertyMappingCoversQueryableProperties(t *testing.T) {
	reachable := make(map[string]bool)
	for iri, property := range oslcPropertyMappings {
		if !IsProperty(property) {
			t.Errorf("OSLC mapping %q = %q, not in property table", iri, property)
		}
		reachable[property] = true
	}
	for _, property := range PropertyNames() {
		if property != PropertyID && !reachable[property] {
			t.Errorf("property %q has no OSLC mapping", property)
		}
	}
	if property, err := resolveProperty("sysml:qualifiedName", DefaultPrefixes); err != nil || property != PropertyQualifiedName {
		t.Fatalf("sysml:qualifiedName mapping = %q, %v", property, err)
	}
	if !reachable[PropertyQualifiedName] {
		t.Fatal("sysml:qualifiedName does not cover element identity")
	}
	rebound := clonePrefixes(DefaultPrefixes)
	rebound["sysml"] = "urn:custom:"
	if _, err := resolveProperty("sysml:name", rebound); err == nil {
		t.Fatal("rebound sysml prefix unexpectedly selected a built-in property")
	}
}

func TestParseParametersPreservesEncodedSemicolonAndDecodesValues(t *testing.T) {
	q, err := ParseParameters(`oslc.where=sysml%3Aname%3D%22a%253Bb%2Bc%22`)
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Where[0].Values[0]; got != "a%3Bb+c" {
		t.Fatalf("value = %q, want one URL-decoding pass", got)
	}
	q, err = ParseParameters(`oslc.where=sysml%3Aname%3D%22a%3Bb%2Bc%22`)
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Where[0].Values[0]; got != "a;b+c" {
		t.Fatalf("value = %q, want semicolon preserved", got)
	}
	q, err = ParseParameters(`oslc.where=sysml%3Aname%3D%22a+b%22`)
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Where[0].Values[0]; got != "a b" {
		t.Fatalf("value = %q, want plus decoded as space", got)
	}
}

func TestParseOSLCMalformedNeverPanics(t *testing.T) {
	inputs := []string{"", `sysml:name=`, `sysml:name="`, `sysml:name in [`, `sysml:name="x" or sysml:type="y"`, `oslc.select=sysml:name,`}
	for _, input := range inputs {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("ParseOSLC(%q) panicked: %v", input, recovered)
				}
			}()
			_, _ = ParseOSLC(input)
		}()
	}
}
