package passes

import "testing"

// MetadataUsage_Invalid.sysml.xt: a metadata body feature that redefines no
// feature of the metadata definition is an error at the feature.
func TestW8DMetadataBodyFeatureMustRedefineOwningTypeFeature(t *testing.T) {
	src := `package Test {
	metadata def A {
		attribute x;
		attribute u {
			attribute v;
		}
	}
	attribute bad;
	attribute a {
		@A {
			x = 1;
			u {
				v = 1;
				bad;
			}
			other;
		}
	}
}
`
	w8dWantLines(t, src, "metadata-body-feature", 14, 16)
}

// The reference's INVALID_METADATA_FEATURE_METACLASS_NOT_ABSTRACT: annotating
// with an abstract metadata definition has no concrete type.
func TestW8DMetadataAnnotationTypeMustBeConcrete(t *testing.T) {
	src := `package Test {
	abstract metadata def Abs;
	metadata def Concrete;
	item p {
		@Abs;
		@Concrete;
	}
}
`
	w8dWantLines(t, src, "metadata-abstract-type", 5)
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
	}
}
`
	for _, code := range []string{"metadata-body-feature", "metadata-abstract-type"} {
		if lines := w8dLines(t, src, code); len(lines) != 0 {
			t.Fatalf("%s on a legal model at lines %v", code, lines)
		}
	}
}
