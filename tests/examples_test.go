package tests

import (
	"sort"
	"strings"
	"testing"
)

func rules(rep jsonReport) []string {
	var out []string
	for _, f := range rep.Findings {
		out = append(out, f.Severity+" "+f.Rule)
	}
	sort.Strings(out)
	return out
}

func TestExamples(t *testing.T) {
	cases := []struct {
		path string
		code int
		want string
	}{
		{"examples/broken/checkout.yaml", 1, "HIGH LS101,HIGH LS102,HIGH LS501,MEDIUM LS107,MEDIUM LS303,MEDIUM LS401,MEDIUM LS504,MEDIUM LS601"},
		{"examples/fixed/checkout.yaml", 0, ""},
		{"examples/compose/compose.yaml", 1, "HIGH LS103,HIGH LS104,MEDIUM LS202,MEDIUM LS403"},
	}
	for _, c := range cases {
		rep, r := runJSON(t, nil, c.path)
		if r.code != c.code {
			t.Errorf("%s: exit %d, want %d\n%s", c.path, r.code, c.code, r.stderr)
		}
		if got := strings.Join(rules(rep), ","); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.path, got, c.want)
		}
	}
}

// Real configs from projects that run them must stay quiet.
func TestRealConfigsQuiet(t *testing.T) {
	rep, r := runJSON(t, nil, "--fail-on", "low", "tests/testdata/real")
	if r.code != 0 || len(rep.Findings) != 0 {
		t.Fatalf("exit %d, findings %v\n%s", r.code, rules(rep), r.stderr)
	}
	if rep.Summary.Files != 4 || rep.Summary.Variables < 250 {
		t.Errorf("read less than expected: %+v", rep.Summary)
	}
}

func TestFailOnLevels(t *testing.T) {
	want := map[string]int{"high": 1, "medium": 1, "low": 1, "never": 0}
	for level, code := range want {
		if r := runTool(t, nil, "--fail-on", level, "examples/broken/checkout.yaml"); r.code != code {
			t.Errorf("--fail-on %s: exit %d, want %d", level, r.code, code)
		}
	}
	// Only medium findings: high passes, medium fails.
	r := runTool(t, []byte("OTEL_TRACES_SAMPLER_ARG=0.1\n"), "--fail-on", "high", "-")
	if r.code != 2 {
		t.Errorf("stdin is read as YAML, so an env line is refused: exit %d", r.code)
	}
}
