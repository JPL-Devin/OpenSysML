package conformance

import "testing"

// The default mode is the zero value, so a caller that names no mode gets the
// behavior it had before the option existed.
func TestDefaultIsTheZeroValue(t *testing.T) {
	var mode Mode
	if mode != ModeDefault || mode.IsStrict() {
		t.Fatalf("zero value = %v, want the default mode", mode)
	}
}

func TestModeStrings(t *testing.T) {
	for mode, want := range map[Mode]string{ModeDefault: "default", ModeStrict: "strict", Mode(7): "Mode(7)"} {
		if got := mode.String(); got != want {
			t.Errorf("Mode(%d).String() = %q, want %q", int(mode), got, want)
		}
	}
}

func TestModeOf(t *testing.T) {
	if ModeOf(true) != ModeStrict || ModeOf(false) != ModeDefault {
		t.Fatal("ModeOf must map true to strict and false to default")
	}
}

func TestParseMode(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    Mode
		wantErr bool
	}{
		{in: "", want: ModeDefault},
		{in: "default", want: ModeDefault},
		{in: "strict", want: ModeStrict},
		{in: "Strict", wantErr: true},
		{in: "lenient", wantErr: true},
	} {
		got, err := ParseMode(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseMode(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			continue
		}
		if err == nil && got != tc.want {
			t.Errorf("ParseMode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
