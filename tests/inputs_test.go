package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var good = "examples/fixed/checkout.yaml"

func badFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(root("tests", "testdata", "bad"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, "tests/testdata/bad/"+e.Name())
		}
	}
	if len(out) < 30 {
		t.Fatalf("only %d bad files", len(out))
	}
	return out
}

func expectRefused(t *testing.T, r result, what string) {
	t.Helper()
	if r.code != 2 {
		t.Errorf("%s: exit %d, want 2\nstdout: %s\nstderr: %s", what, r.code, r.stdout, r.stderr)
	}
	if r.stdout != "" {
		t.Errorf("%s: printed to stdout:\n%s", what, r.stdout)
	}
	if !strings.Contains(r.stderr, "can't trust the input") {
		t.Errorf("%s: stderr doesn't say why: %q", what, r.stderr)
	}
	if strings.Contains(r.stdout+r.stderr, "No problems") {
		t.Errorf("%s: printed a clean result", what)
	}
}

// Every bad file is refused alone, next to a good file in either order, on
// stdin, under every --fail-on and in both formats.
func TestBadInputsRefused(t *testing.T) {
	for _, f := range badFiles(t) {
		for _, failOn := range []string{"high", "medium", "low", "never"} {
			for _, format := range []string{"text", "json"} {
				flags := []string{"--fail-on", failOn, "--format", format}
				expectRefused(t, runTool(t, nil, append(flags, f)...), f+" "+failOn+" "+format)
				if strings.HasSuffix(f, "no-otel.yaml") {
					continue // valid on its own; refused only because nothing could be checked
				}
				expectRefused(t, runTool(t, nil, append(flags, good, f)...), "good then "+f)
				expectRefused(t, runTool(t, nil, append(flags, f, good)...), f+" then good")
			}
		}
		if strings.HasSuffix(f, ".env") {
			continue // env files are known by name, so stdin is read as YAML
		}
		data, err := os.ReadFile(root(f))
		if err != nil {
			t.Fatal(err)
		}
		expectRefused(t, runTool(t, data, "-"), f+" on stdin")
	}
}

// A bad file inside a directory of good ones fails the whole run.
func TestBadFileInDirectory(t *testing.T) {
	refused, skipped := 0, 0
	for _, f := range badFiles(t) {
		dir := t.TempDir()
		copyFile(t, root(good), filepath.Join(dir, "checkout.yaml"))
		name := filepath.Base(f)
		switch filepath.Ext(name) {
		case ".yaml", ".yml", ".json", ".env":
		default:
			name += ".yaml" // as if the failed command's output was saved as a manifest
		}
		copyFile(t, root(f), filepath.Join(dir, name))
		r := runTool(t, nil, dir)
		switch name {
		case "no-otel.yaml":
			// A workload without OTEL_ variables is fine next to ones that have them.
			if r.code != 0 {
				t.Errorf("%s in a directory: exit %d\n%s%s", name, r.code, r.stdout, r.stderr)
			}
		case "github-workflow.yaml", "helm-values.yaml", "json-other.json", "comments-only.yaml", "helm-unrendered.yaml":
			// Valid YAML that isn't config, and templates, are skipped and listed.
			if r.code != 0 || !strings.Contains(r.stdout, "Skipped 1 file") {
				t.Errorf("%s in a directory: exit %d\n%s%s", name, r.code, r.stdout, r.stderr)
			}
			skipped++
		default:
			expectRefused(t, r, name+" in a directory")
			refused++
		}
	}
	if refused != 29 || skipped != 5 {
		t.Errorf("refused %d and skipped %d; the README says 29 and 5", refused, skipped)
	}
}

// Compose reads two more files: the env_file and the .env used for ${VAR}.
// Each one bad, with the compose file real, is refused.
func TestComposeSideFilesRefused(t *testing.T) {
	bad := map[string]string{
		"empty":    "",
		"garbage":  "this is not an env file\n",
		"cut":      "OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-coll",
		"failed":   "Error: open otel.env: no such file or directory\n",
		"binary":   "\x00\x01\x02OTEL\n",
		"unclosed": "OTEL_SERVICE_NAME=\"shop\n",
	}
	for name, content := range bad {
		for _, side := range []string{"otel.env", ".env"} {
			dir := t.TempDir()
			copyFile(t, root("examples/compose/compose.yaml"), filepath.Join(dir, "compose.yaml"))
			copyFile(t, root("examples/compose/otel.env"), filepath.Join(dir, "otel.env"))
			if err := os.WriteFile(filepath.Join(dir, side), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, failOn := range []string{"high", "never"} {
				expectRefused(t, runTool(t, nil, "--fail-on", failOn, filepath.Join(dir, "compose.yaml")), name+" "+side)
			}
		}
	}
}

func TestBadFlags(t *testing.T) {
	for _, args := range [][]string{{}, {"--fail-on", "error", good}, {"--format", "sarif", good}, {"--nope", good}, {"-", "-"}} {
		r := runTool(t, []byte(""), args...)
		if r.code != 2 || r.stdout != "" {
			t.Errorf("%v: exit %d, stdout %q", args, r.code, r.stdout)
		}
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
