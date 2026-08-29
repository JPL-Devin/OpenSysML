package passes

import "testing"

func TestW8CMetadataAbstractType(t *testing.T) {
	src := `package P {
	abstract metaclass A { feature :>> annotatedElement : KerML::Classifier; }
	classifier B {
		@A;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "meta-abstract.kerml", src)
	if w8cCount(msgs, msgMetadataConcreteType) != 1 {
		t.Errorf("want one %q, got %v", msgMetadataConcreteType, msgs)
	}
}

// The rule judges each annotation on its own, so an unresolved name elsewhere in
// the file does not hide it, and an unresolved metaclass draws no cascade.
func TestW8CMetadataAbstractTypeIsElementScoped(t *testing.T) {
	src := `package P {
	abstract metaclass A { feature :>> annotatedElement : KerML::Classifier; }
	classifier B {
		@A;
		feature :>> nowhere;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "meta-abstract-unresolved.kerml", src)
	if w8cCount(msgs, msgMetadataConcreteType) != 1 {
		t.Errorf("want one %q despite the unresolved feature, got %v", msgMetadataConcreteType, msgs)
	}

	unresolved := `package P {
	classifier B {
		@Nowhere;
	}
}`
	msgs = w8cLibraryMessagesIn(t, "meta-unresolved-metaclass.kerml", unresolved)
	if w8cCount(msgs, msgMetadataConcreteType) != 0 {
		t.Errorf("unresolved metaclass must not cascade: %v", msgs)
	}
}

// p24: the metaclass is a library one, so the rule can only judge it while the
// library is parsed on every load path rather than reduced to facts.
func TestW8CMetadataAbstractLibraryMetaclass(t *testing.T) {
	src := `package P {
	item p {
		@Metaobjects::Metaobject;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "meta-abstract-library.sysml", src)
	if w8cCount(msgs, msgMetadataConcreteType) != 1 {
		t.Errorf("want one %q for the abstract library metaclass, got %v", msgMetadataConcreteType, msgs)
	}
}

// A concrete library metaclass stays legal, so the rule reads abstractness rather
// than rejecting library metaclasses.
func TestW8CMetadataConcreteLibraryMetaclass(t *testing.T) {
	src := `package P {
	item p {
		@KerML::Comment;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "meta-concrete-library.sysml", src)
	if w8cCount(msgs, msgMetadataConcreteType) != 0 {
		t.Errorf("unexpected %q for a concrete library metaclass: %v", msgMetadataConcreteType, msgs)
	}
}

func TestW8CMetadataConcreteTypeLegal(t *testing.T) {
	src := `package P {
	metaclass A { feature :>> annotatedElement : KerML::Classifier; }
	classifier B {
		@A;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "meta-concrete.kerml", src)
	if w8cCount(msgs, msgMetadataConcreteType) != 0 {
		t.Errorf("unexpected %q: %v", msgMetadataConcreteType, msgs)
	}
}
