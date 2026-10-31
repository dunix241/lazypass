package shared

import "testing"

func TestDeleteLastRune(t *testing.T) {
	for _, test := range []struct{ input, want string }{{"", ""}, {"a", ""}, {"hello世", "hello"}, {"e\u0301", "e"}} {
		if got := DeleteLastRune(test.input); got != test.want {
			t.Fatalf("DeleteLastRune(%q)=%q want %q", test.input, got, test.want)
		}
	}
}
