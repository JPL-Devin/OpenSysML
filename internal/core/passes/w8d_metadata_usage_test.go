package passes

import "testing"

// MetadataUsage_Invalid.sysml.xt: a metadata body feature that redefines no
// feature of the metadata definition is an error at the feature. Prefix
// annotations are RedefinitionConformancePass's; the `metadata … : A` form is
// this pass's.
func TestW8DMetadataBodyFeatureMustRedefineOwningTypeFeature(t *testing.T) {
	src := `package Test {
	metadata def A {
		attribute x;
		attribute u {
			attribute v;
		}
	}
	item p;
	metadata m : A about p {
		x = 1;
		u {
			v = 1;
			bad;
		}
		other;
	}
}
`
	w8dWantLines(t, src, "metadata-body-feature", 13, 15)
}

// The reference's INVALID_METADATA_FEATURE_METACLASS_NOT_ABSTRACT on the
// `metadata … : A` form; either spelling is reported exactly once.
func TestW8DMetadataUsageTypeMustBeConcrete(t *testing.T) {
	src := `package Test {
	abstract metadata def Abs;
	metadata def Concrete;
	item p;
	metadata m : Abs about p;
	metadata n : Concrete about p;
}
`
	w8dWantLines(t, src, "metadata-concrete-type", 5)

	prefix := `package Test {
	abstract metadata def Abs;
	item p {
		@Abs;
	}
}
`
	var concrete []Diagnostic
	for _, d := range w8dDiags(t, prefix) {
		if d.Message == msgMetadataConcreteType {
			concrete = append(concrete, d)
		}
	}
	if len(concrete) != 1 {
		t.Fatalf("prefix annotation: got %d concrete-type diagnostics, want 1", len(concrete))
	}
}

// An explicit `:>>` in a usage body must name a feature of the metadata
// definition or of one it specializes, nested bodies included.
func TestW8DMetadataUsageBodyRedefinitionMustNameAnOwningTypeFeature(t *testing.T) {
	src := `package Test {
	attribute g;
	metadata def Base { attribute inherited; }
	metadata def A :> Base {
		attribute x;
		attribute u {
			attribute v;
		}
	}
	item p { attribute own; }
	metadata m : A about p {
		:>> x = 1;
		:>> inherited = 2;
		:>> g = 3;
		:>> p::own = 4;
		u {
			:>> v = 5;
			:>> g;
		}
	}
}
`
	w8dWantLines(t, src, "metadata-body-feature", 14, 15, 18)
}

// A metadata usage body binds model-level evaluable values, at any nesting depth
// (validateMetadataFeatureBody, Expression::isModelLevelEvaluable).
func TestW8DMetadataUsageBodyValuesMustBeModelLevelEvaluable(t *testing.T) {
	src := `package Test {
	metadata def A {
		attribute x;
		attribute y;
		attribute u {
			attribute v;
			attribute w;
		}
	}
	item p { attribute own; }
	metadata m : A about p {
		:>> x = ~3;
		:>> y = 1 + 2;
		u {
			:>> v = ~3;
			:>> w = 4;
		}
	}
}
`
	w8dWantLines(t, src, "metadata-value-not-evaluable", 12, 15)
}

func TestW8DLegalMetadataAnnotationsStaySilent(t *testing.T) {
	src := `package Test {
	metadata def A {
		attribute x;
		attribute u {
			attribute v;
		}
	}
	metadata def B :> A;
	item p {
		@A {
			x = 1;
			u {
				v = 2;
			}
		}
	}
	item q {
		@B {
			x = 3;
		}
	}
	metadata m : A about p {
		x = 4;
		u {
			v = 5 + 1;
		}
	}
}
`
	for _, code := range []string{"metadata-body-feature", "metadata-concrete-type", "metadata-value-not-evaluable"} {
		if lines := w8dLines(t, src, code); len(lines) != 0 {
			t.Fatalf("%s on a legal model at lines %v", code, lines)
		}
	}
}
