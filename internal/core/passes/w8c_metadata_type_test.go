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
