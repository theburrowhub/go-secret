package sources

import (
	"bytes"
	"strings"
	"testing"
)

func TestPromptForSource_PicksByNumber(t *testing.T) {
	in := strings.NewReader("2\n")
	out := &bytes.Buffer{}
	picked, err := promptForSourceFromIO(in, out, []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if picked != "b" {
		t.Fatalf("got %q", picked)
	}
	if !strings.Contains(out.String(), "1) a") {
		t.Fatalf("listing missing: %q", out.String())
	}
}

func TestPromptForSource_RejectsOutOfRange(t *testing.T) {
	in := strings.NewReader("9\n1\n")
	out := &bytes.Buffer{}
	picked, err := promptForSourceFromIO(in, out, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if picked != "a" {
		t.Fatalf("got %q", picked)
	}
}
