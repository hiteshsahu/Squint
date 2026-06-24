package source

import "testing"

func TestExplain(t *testing.T) {
	// Known codes must produce a non-empty explanation.
	for _, code := range []string{"Resources", "Priority", "QOSMaxGRESPerUser", "Dependency", "ReqNodeNotAvail"} {
		if got := Explain(code); got.Plain == "" {
			t.Errorf("Explain(%q) returned empty Plain", code)
		}
	}
	// Empty / None reads as "being scheduled" with no suggestion.
	if got := Explain(""); got.Plain == "" || got.Suggestion != "" {
		t.Errorf("Explain(\"\") = %+v, want non-empty Plain and empty Suggestion", got)
	}
	// Unknown code is echoed back rather than dropped.
	if got := Explain("SomeNovelReason"); got.Plain != "SomeNovelReason" {
		t.Errorf("Explain unknown = %q, want the reason echoed", got.Plain)
	}
}
