package rewrite

import "testing"

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
