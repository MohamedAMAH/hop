package claude

import "testing"

func TestEncodeDir(t *testing.T) {
	cases := map[string]string{
		`D:\Fun\Projects\hop`: "D--Fun-Projects-hop",
		`/home/abdel/hop`:     "-home-abdel-hop",
		`dot.test`:            "dot-test",
		`under_score`:         "under-score",
		`with space`:          "with-space",
		`v1.2.3`:              "v1-2-3",
		`a-b`:                 "a-b",
		`UPPER`:               "UPPER",
		`plus+eq=at@`:         "plus-eq-at-",
	}
	for in, want := range cases {
		if got := EncodeDir(in); got != want {
			t.Errorf("EncodeDir(%q) = %q, want %q", in, got, want)
		}
	}
}
