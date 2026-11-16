// Command lostspans checks OpenTelemetry SDK settings (OTEL_ environment
// variables) in Kubernetes manifests, Docker Compose files and env files, and
// reports the ones that make telemetry go nowhere: wrong OTLP endpoint paths,
// invalid protocols, timeouts in seconds instead of milliseconds, malformed
// headers and more. It reads files only.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
)

const version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

const usage = `usage: lostspans [--fail-on high|medium|low|never] [--format text|json] PATH...

Checks OTEL_ variables in Kubernetes manifests (YAML or JSON, helm template
output, kubectl get -o yaml), Docker Compose files and .env files. PATH can be
a file, a directory, or - for stdin.

Exit codes: 0 nothing at or above --fail-on (default high), 1 findings at or
above it, 2 bad flags or input that can't be trusted (nothing was checked).
`

type summary struct {
	Files      int `json:"files"`
	Containers int `json:"containers"`
	Services   int `json:"services"`
	EnvFiles   int `json:"envFiles"`
	ConfigMaps int `json:"configMaps"`
	Objects    int `json:"objects"`
	Scopes     int `json:"scopes"`
	Variables  int `json:"variables"`
	Unchecked  int `json:"unchecked"`
	High       int `json:"high"`
	Medium     int `json:"medium"`
	Low        int `json:"low"`
}

// unchecked is a variable whose value isn't in the files, or a source that wasn't read.
type unchecked struct {
	Scope    string `json:"scope"`
	Variable string `json:"variable,omitempty"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Reason   string `json:"reason"`
}

type report struct {
	Findings  []Finding   `json:"findings"`
	Unchecked []unchecked `json:"unchecked"`
	Skipped   []string    `json:"skipped"`
	Summary   summary     `json:"summary"`
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("lostspans", flag.ContinueOnError)
	fl.SetOutput(io.Discard)
	failOn := fl.String("fail-on", "high", "")
	format := fl.String("format", "text", "")
	showVersion := fl.Bool("version", false, "")
	if err := fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		fmt.Fprintf(stderr, "lostspans: %v\n%s", err, usage)
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, "lostspans", version)
		return 0
	}
	threshold, ok := map[string]Severity{"high": High, "medium": Medium, "low": Low, "never": High + 1}[*failOn]
	if !ok {
		fmt.Fprintf(stderr, "lostspans: --fail-on must be high, medium, low or never, not %q\n", *failOn)
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "lostspans: --format must be text or json, not %q\n", *format)
		return 2
	}
	if fl.NArg() == 0 {
		fmt.Fprintf(stderr, "lostspans: name at least one file or directory\n%s", usage)
		return 2
	}
	rep, err := analyze(fl.Args(), stdin)
	if err != nil {
		fmt.Fprintf(stderr, "lostspans: can't trust the input, nothing was checked: %v\n", err)
		return 2
	}
	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(rep)
	} else {
		printText(stdout, rep)
	}
	for _, f := range rep.Findings {
		if f.Severity >= threshold {
			return 1
		}
	}
	return 0
}

func analyze(paths []string, stdin io.Reader) (*report, error) {
	srcs, err := readArgs(paths, stdin)
	if err != nil {
		return nil, err
	}
	inv, err := load(srcs)
	if err != nil {
		return nil, err
	}
	rep := &report{Findings: []Finding{}, Unchecked: []unchecked{}, Skipped: []string{}}
	for _, s := range inv.Skipped {
		rep.Skipped = append(rep.Skipped, s.file+" ("+s.reason+")")
	}
	if inv.Files == 0 {
		return nil, errors.New("no Kubernetes manifests, Compose files or env files were found in " + strings.Join(paths, ", "))
	}
	scopes := 0
	vars := 0
	for _, sc := range inv.Scopes {
		if len(sc.Vars) == 0 {
			continue
		}
		scopes++
		switch sc.Platform {
		case "kubernetes":
			rep.Summary.Containers++
		case "compose":
			rep.Summary.Services++
		case "envfile":
			rep.Summary.EnvFiles++
		case "configmap":
			rep.Summary.ConfigMaps++
		}
		vars += len(sc.Vars)
		for _, v := range sc.Vars {
			if !v.Known {
				rep.Unchecked = append(rep.Unchecked, unchecked{sc.Label, v.Name, v.File, v.Line, v.Why})
			}
		}
		for _, u := range sc.Unread {
			rep.Unchecked = append(rep.Unchecked, unchecked{Scope: sc.Label, File: sc.File, Line: sc.Line, Reason: u + " isn't in the inputs, so its variables weren't read"})
		}
		rep.Findings = append(rep.Findings, check(sc)...)
	}
	if vars == 0 {
		return nil, fmt.Errorf("no OTEL_ variables were found in %s with %s, so there was nothing to check. If they come from a ConfigMap, Secret or .env file, pass that too", plural(inv.Files, "file", "files"), plural(inv.Objects, "object", "objects"))
	}
	rep.Summary.Files, rep.Summary.Objects, rep.Summary.Scopes = inv.Files, inv.Objects, scopes
	rep.Summary.Variables, rep.Summary.Unchecked = vars, len(rep.Unchecked)
	for _, f := range rep.Findings {
		switch f.Severity {
		case High:
			rep.Summary.High++
		case Medium:
			rep.Summary.Medium++
		default:
			rep.Summary.Low++
		}
	}
	return rep, nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func printText(w io.Writer, rep *report) {
	for _, f := range rep.Findings {
		fmt.Fprintf(w, "%-6s %s %s (%s:%d)\n", f.Level, f.Rule, f.Scope, f.File, f.Line)
		fmt.Fprintf(w, "  %s=%s\n", f.Variable, f.Value)
		fmt.Fprintf(w, "  %s\n", f.Message)
		fmt.Fprintf(w, "  fix: %s\n\n", f.Fix)
	}
	s := rep.Summary
	var where []string
	for _, p := range []struct {
		n         int
		one, many string
	}{{s.Containers, "container", "containers"}, {s.Services, "Compose service", "Compose services"},
		{s.EnvFiles, "env file", "env files"}, {s.ConfigMaps, "ConfigMap", "ConfigMaps"}} {
		if p.n > 0 {
			where = append(where, plural(p.n, p.one, p.many))
		}
	}
	read := fmt.Sprintf("checked %s in %s, from %s", plural(s.Variables, "OTEL_ variable", "OTEL_ variables"),
		joinAnd(where), plural(s.Files, "file", "files"))
	if len(rep.Findings) == 0 {
		fmt.Fprintf(w, "No problems: %s.\n", read)
	} else {
		fmt.Fprintf(w, "%d high, %d medium, %d low: %s.\n", s.High, s.Medium, s.Low, read)
	}
	printUnchecked(w, rep.Unchecked)
	if len(rep.Skipped) > 0 {
		fmt.Fprintf(w, "Skipped %s in directories: %s\n", plural(len(rep.Skipped), "file", "files"), strings.Join(rep.Skipped, "; "))
	}
}

// printUnchecked lists sources that weren't read, and counts values that
// aren't in the files by reason. --format json lists every one.
func printUnchecked(w io.Writer, list []unchecked) {
	counts := map[string]int{}
	var order []string
	for _, u := range list {
		if u.Variable == "" {
			fmt.Fprintf(w, "Not read: %s: %s.\n", u.Scope, u.Reason)
			continue
		}
		reason := u.Reason
		if strings.HasPrefix(reason, "uses ") {
			reason = "built from a variable whose value isn't in the files"
		}
		if counts[reason] == 0 {
			order = append(order, reason)
		}
		counts[reason]++
	}
	if len(order) == 0 {
		return
	}
	var parts []string
	for _, r := range order {
		parts = append(parts, fmt.Sprintf("%d %s", counts[r], r))
	}
	fmt.Fprintf(w, "Not checked, because the value isn't in the files: %s. --format json lists them.\n", strings.Join(parts, ", "))
}

func readFileIfExists(p string) ([]byte, error) {
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, untrusted{p, "can't read: " + plainErr(err)}
	}
	return data, nil
}
