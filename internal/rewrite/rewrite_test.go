package rewrite

import (
	"testing"

	"hop/internal/osinfo"
)

func TestRewriteJSONStringsPreservesStructureAndEscapes(t *testing.T) {
	src := []byte(`{"cwd":"D:\\Fun\\hop","n":5}` + "\n" + `{"b":"x"}`)
	// Uppercase every decoded string value; keys are strings too, so they
	// change — that is fine for this structural test.
	out, err := rewriteJSONStrings(src, func(s string) string {
		return s + "!"
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"cwd!":"D:\\Fun\\hop!","n!":5}` + "\n" + `{"b!":"x!"}`
	if string(out) != want {
		t.Fatalf("got  %s\nwant %s", out, want)
	}
}

func TestRewriteJSONStringsHandlesEscapedQuote(t *testing.T) {
	src := []byte(`{"k":"a\"b"}`)
	out, err := rewriteJSONStrings(src, func(s string) string { return s })
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(src) {
		t.Fatalf("identity transform changed bytes: got %s want %s", out, src)
	}
}

func TestRewriteJSONStringsPreservesHTMLCharacters(t *testing.T) {
	src := []byte(`{"cmd":"a && b < c > d"}`)
	out, err := rewriteJSONStrings(src, func(s string) string { return s })
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(src) {
		t.Fatalf("identity transform changed bytes: got %s want %s", out, src)
	}
}

func TestNeutralizeWindows(t *testing.T) {
	src := []byte(`{"cwd":"D:\\Fun\\hop","msg":"see D:\\Fun\\hop\\src\\main.go here"}`)
	out, err := Neutralize(src, `D:\Fun\hop`, osinfo.Windows, "__HOP_ROOT__")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"cwd":"__HOP_ROOT__","msg":"see __HOP_ROOT__/src/main.go here"}`
	if string(out) != want {
		t.Fatalf("got  %s\nwant %s", out, want)
	}
}

func TestNeutralizeDoesNotCorruptCoincidentalText(t *testing.T) {
	// "hope" contains "hop" but is not the root at a boundary+tail; must survive.
	src := []byte(`{"msg":"I hope /home/x/hop works"}`)
	out, err := Neutralize(src, `/home/x/hop`, osinfo.Unix, "__HOP_ROOT__")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"msg":"I hope __HOP_ROOT__ works"}`
	if string(out) != want {
		t.Fatalf("got  %s\nwant %s", out, want)
	}
}

func TestNeutralizeDoesNotMatchLongerSiblingPathSegment(t *testing.T) {
	// "/home/x/hopfoo" shares a prefix with the root but is a different
	// directory; the root must not match unless it ends at a path boundary.
	src := []byte(`{"msg":"see /home/x/hopfoo/file.go"}`)
	out, err := Neutralize(src, `/home/x/hop`, osinfo.Unix, "__HOP_ROOT__")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"msg":"see /home/x/hopfoo/file.go"}`
	if string(out) != want {
		t.Fatalf("got  %s\nwant %s", out, want)
	}
}
