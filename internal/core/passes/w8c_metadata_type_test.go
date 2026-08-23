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
	if got := w8cCount(msgs, msgMetadataConcreteType); got != 1 {
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
	if got := w8cCount(msgs, msgMetadataConcreteType); got != 1 {
		t.Errorf("want one %q despite the unresolved feature, got %v", msgMetadataConcreteType, msgs)
	}

	unresolved := `package P {
	classifier B {
		@Nowhere;
	}
}`
	msgs = w8cLibraryMessagesIn(t, "meta-unresolved-metaclass.kerml", unresolved)
	if got := w8cCount(msgs, msgMetadataConcreteType); got != 0 {
		t.Errorf("unresolved metaclass must not cascade: %v", msgs)
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
	if got := w8cCount(msgs, msgMetadataConcreteType); got != 0 {
		t.Errorf("unexpected %q: %v", msgMetadataConcreteType, msgs)
	}
}
