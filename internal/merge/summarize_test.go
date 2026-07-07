package merge

import (
	"reflect"
	"testing"
)

func TestSummarize(t *testing.T) {
	local := []byte("a\nb\nc\n")
	incoming := []byte("a\nb\nX\nY\n")
	got := Summarize(local, incoming)
	want := DiffSummary{
		ForkLine:      2,
		LocalLines:    3,
		IncomingLines: 4,
		LocalTail:     []string{"c"},
		IncomingTail:  []string{"X", "Y"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Summarize = %+v, want %+v", got, want)
	}
}

func TestSummarizeIdentical(t *testing.T) {
	s := Summarize([]byte("a\nb\n"), []byte("a\nb\n"))
	if s.ForkLine != 2 || len(s.LocalTail) != 0 || len(s.IncomingTail) != 0 {
		t.Fatalf("identical inputs should have no tails, got %+v", s)
	}
}
