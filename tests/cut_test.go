package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cutTargets are complete files to cut. step is the byte stride.
var cutTargets = []struct {
	path string
	step int
}{
	{"examples/broken/checkout.yaml", 3},
	{"examples/fixed/checkout.yaml", 3},
	{"examples/compose/otel.env", 1},
	{"tests/testdata/real/jaeger-hotrod/docker-compose.yml", 5},
	{"tests/testdata/real/opentelemetry-demo/compose.yaml", 97},
	{"tests/testdata/real/opentelemetry-demo/.env", 41},
	{"tests/testdata/real/opentelemetry-helm-charts/opentelemetry-demo-rendered.yaml", 211},
}

// A file cut in the middle of a line is always refused. A file cut at a line
// break is refused, or read as a shorter file with no more variables than the
// whole one. No cut may report a clean result for more than the file holds.
func TestCutFiles(t *testing.T) {
	for _, target := range cutTargets {
		data, err := os.ReadFile(root(target.path))
		if err != nil {
			t.Fatal(err)
		}
		full, r := runJSON(t, nil, "--fail-on", "never", target.path)
		if r.code != 0 {
			t.Fatalf("%s: whole file exit %d: %s", target.path, r.code, r.stderr)
		}
		name := filepath.Base(target.path)
		dir := t.TempDir()
		cut := filepath.Join(dir, name)
		refusedLines, readLines := 0, 0
		for i := 1; i < len(data); i++ {
			atLineBreak := data[i-1] == '\n'
			if !atLineBreak && i%target.step != 0 {
				continue
			}
			if err := os.WriteFile(cut, data[:i], 0o644); err != nil {
				t.Fatal(err)
			}
			rep, r := runJSON(t, nil, "--fail-on", "never", cut)
			if !atLineBreak {
				if r.code != 2 || r.stdout != "" {
					t.Errorf("%s cut at byte %d: exit %d, want 2\n%s", target.path, i, r.code, r.stdout)
				}
				continue
			}
			if r.code == 2 {
				refusedLines++
				continue
			}
			readLines++
			if rep.Summary.Variables > full.Summary.Variables {
				t.Errorf("%s cut at byte %d: %d variables, more than the whole file's %d", target.path, i, rep.Summary.Variables, full.Summary.Variables)
			}
		}
		t.Logf("%s: line cuts refused %d, read as shorter files %d", target.path, refusedLines, readLines)
	}
}

// The text output of a cut file never claims more than it read.
func TestCutSummaryShowsShortRead(t *testing.T) {
	data, err := os.ReadFile(root("examples/fixed/checkout.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(string(data), "\n")
	cut := filepath.Join(t.TempDir(), "checkout.yaml")
	os.WriteFile(cut, []byte(strings.Join(lines[:36], "")), 0o644)
	r := runTool(t, nil, cut)
	if r.code == 0 && !strings.Contains(r.stdout, "in 1 container") {
		t.Errorf("a file cut after the first workload should say it read 1 container:\n%s", r.stdout)
	}
}
