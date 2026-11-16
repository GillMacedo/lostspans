// Package sdk runs the real OpenTelemetry Go SDK with each configuration that
// lostspans reports, and with healthy ones, and checks that the two agree:
// lostspans reports the rule exactly when the SDK loses telemetry or sends it
// somewhere other than intended. It needs network access once, to download
// the SDK modules, so it lives in its own module outside `make test`.
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type request struct {
	path    string
	headers http.Header
}

// collector is an OTLP/HTTP stand-in that accepts only /v1/traces.
type collector struct {
	mu    sync.Mutex
	reqs  []request
	delay time.Duration
	srv   *httptest.Server
}

func newCollector(delay time.Duration) *collector {
	c := &collector{delay: delay}
	c.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		time.Sleep(c.delay)
		c.mu.Lock()
		c.reqs = append(c.reqs, request{r.URL.Path, r.Header.Clone()})
		c.mu.Unlock()
		if r.URL.Path != "/v1/traces" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	return c
}

func (c *collector) delivered() []request {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []request
	for _, r := range c.reqs {
		if r.path == "/v1/traces" {
			out = append(out, r)
		}
	}
	return out
}

// outcome is what the SDK did with one span.
type outcome struct {
	exporterErr error
	flushErr    error
	resourceErr error
	resource    *resource.Resource
	delivered   []request
}

func exportOneSpan(t *testing.T, c *collector) outcome {
	t.Helper()
	ctx := context.Background()
	exp, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return outcome{exporterErr: err}
	}
	res, rerr := resource.New(ctx, resource.WithFromEnv())
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
	_, span := tp.Tracer("lostspans-test").Start(ctx, "op")
	span.End()
	fctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ferr := tp.ForceFlush(fctx)
	tp.Shutdown(fctx)
	return outcome{flushErr: ferr, resourceErr: rerr, resource: res, delivered: c.delivered()}
}

var bin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "lostspans-sdk")
	if err != nil {
		panic(err)
	}
	bin = filepath.Join(dir, "lostspans")
	cmd := exec.Command("go", "build", "-o", bin, "./plugin/scripts")
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building lostspans: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type finding struct {
	Severity, Rule, Variable string
}

// lint writes env to an env file and returns lostspans's findings.
func lint(t *testing.T, env [][2]string) []finding {
	t.Helper()
	var b strings.Builder
	for _, kv := range env {
		fmt.Fprintf(&b, "%s=%s\n", kv[0], kv[1])
	}
	f := filepath.Join(t.TempDir(), "service.env")
	if err := os.WriteFile(f, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, "--format", "json", "--fail-on", "never", f).Output()
	if err != nil {
		t.Fatalf("lostspans failed: %v\n%s", err, out)
	}
	var rep struct{ Findings []finding }
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatal(err)
	}
	return rep.Findings
}

type sdkCase struct {
	name  string
	env   [][2]string
	delay time.Duration
	// rule lostspans must report, or "" for a healthy config that must stay quiet.
	rule, severity string
	// lost says how the real SDK fails, and must be true for every reported case.
	lost func(o outcome) bool
}

func noSpan(o outcome) bool { return o.exporterErr != nil || len(o.delivered) == 0 }

func TestSDKAgrees(t *testing.T) {
	cases := []sdkCase{
		{name: "healthy base endpoint", env: [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"}}},
		{name: "healthy traces endpoint with path", env: [][2]string{{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://HOSTPORT/v1/traces"}}},
		{name: "healthy everything", env: [][2]string{
			{"OTEL_SERVICE_NAME", "checkout"},
			{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"},
			{"OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf"},
			{"OTEL_EXPORTER_OTLP_TIMEOUT", "5000"},
			{"OTEL_EXPORTER_OTLP_HEADERS", "Authorization=Bearer%20abc,X-Tenant=t1"},
			{"OTEL_RESOURCE_ATTRIBUTES", "deployment.environment=prod,team=payments"},
			{"OTEL_TRACES_SAMPLER", "parentbased_traceidratio"},
			{"OTEL_TRACES_SAMPLER_ARG", "1.0"},
		}},
		{name: "base endpoint with signal path", env: [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT/v1/traces"}},
			rule: "LS101", severity: "HIGH", lost: noSpan},
		{name: "signal endpoint without path", env: [][2]string{{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://HOSTPORT"}},
			rule: "LS102", severity: "HIGH", lost: noSpan},
		{name: "signal endpoint with another signal's path", env: [][2]string{{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://HOSTPORT/v1/metrics"}},
			rule: "LS103", severity: "HIGH", lost: noSpan},
		{name: "endpoint without scheme", env: [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "HOSTPORT"}},
			rule: "LS104", severity: "HIGH", lost: noSpan},
		{name: "protocol http", env: [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"}, {"OTEL_EXPORTER_OTLP_PROTOCOL", "http"}},
			rule: "LS201", severity: "HIGH", lost: noSpan},
		{name: "protocol upper case", env: [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"}, {"OTEL_EXPORTER_OTLP_PROTOCOL", "HTTP/PROTOBUF"}},
			rule: "LS202", severity: "MEDIUM", lost: noSpan},
		{name: "exporter upper case", env: [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"}, {"OTEL_TRACES_EXPORTER", "OTLP"}},
			rule: "LS202", severity: "MEDIUM", lost: noSpan},
		{name: "jaeger exporter", env: [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"}, {"OTEL_TRACES_EXPORTER", "jaeger"}},
			rule: "LS203", severity: "HIGH", lost: noSpan},
		{name: "timeout in seconds", delay: 200 * time.Millisecond,
			env:  [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"}, {"OTEL_EXPORTER_OTLP_TIMEOUT", "10"}},
			rule: "LS401", severity: "MEDIUM",
			// The SDK cancels the request after 10 ms and counts the export as failed.
			lost: func(o outcome) bool { return o.flushErr != nil }},
		{name: "timeout with unit is ignored", delay: 200 * time.Millisecond,
			env:  [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"}, {"OTEL_EXPORTER_OTLP_TIMEOUT", "1s"}},
			rule: "LS402", severity: "MEDIUM",
			// The setting is ignored and the 10 s default applies, so the span arrives.
			lost: func(o outcome) bool { return len(o.delivered) == 1 }},
		{name: "headers with colon", env: [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"}, {"OTEL_EXPORTER_OTLP_HEADERS", "Authorization: Bearer abc"}},
			rule: "LS501", severity: "HIGH",
			lost: func(o outcome) bool {
				return len(o.delivered) == 1 && o.delivered[0].headers.Get("Authorization") == ""
			}},
		{name: "header without value isn't sent", env: [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"}, {"OTEL_EXPORTER_OTLP_HEADERS", "X-Tenant=t1,Authorization"}},
			rule: "LS501", severity: "HIGH",
			lost: func(o outcome) bool {
				return len(o.delivered) == 1 && o.delivered[0].headers.Get("Authorization") == "" && o.delivered[0].headers.Get("X-Tenant") == "t1"
			}},
		{name: "headers with semicolon", env: [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"}, {"OTEL_EXPORTER_OTLP_HEADERS", "X-Tenant=t1;Authorization=abc"}},
			rule: "LS502", severity: "MEDIUM",
			lost: func(o outcome) bool {
				return len(o.delivered) == 1 && o.delivered[0].headers.Get("Authorization") == "" && o.delivered[0].headers.Get("X-Tenant") != "t1"
			}},
		{name: "resource attribute without value", env: [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"}, {"OTEL_RESOURCE_ATTRIBUTES", "service.name=checkout,team"}},
			rule: "LS503", severity: "MEDIUM", lost: func(o outcome) bool { return o.resourceErr != nil }},
		{name: "resource attributes with semicolon", env: [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"}, {"OTEL_RESOURCE_ATTRIBUTES", "service.name=checkout;team=payments"}},
			rule: "LS504", severity: "MEDIUM",
			lost: func(o outcome) bool {
				v, _ := o.resource.Set().Value(attribute.Key("service.name"))
				return v.AsString() != "checkout"
			}},
		{name: "ratio zero", env: [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"}, {"OTEL_TRACES_SAMPLER", "traceidratio"}, {"OTEL_TRACES_SAMPLER_ARG", "0"}},
			rule: "LS301", severity: "HIGH", lost: noSpan},
		{name: "sampler arg without sampler", env: [][2]string{{"OTEL_EXPORTER_OTLP_ENDPOINT", "http://HOSTPORT"}, {"OTEL_TRACES_SAMPLER_ARG", "0"}},
			rule: "LS303", severity: "MEDIUM",
			// The argument is ignored, so the span is sampled even though the ratio is 0.
			lost: func(o outcome) bool { return len(o.delivered) == 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newCollector(tc.delay)
			defer c.srv.Close()
			hostport := strings.TrimPrefix(c.srv.URL, "http://")
			env := make([][2]string, len(tc.env))
			for _, k := range []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "OTEL_EXPORTER_OTLP_PROTOCOL",
				"OTEL_TRACES_EXPORTER", "OTEL_EXPORTER_OTLP_TIMEOUT", "OTEL_EXPORTER_OTLP_HEADERS", "OTEL_RESOURCE_ATTRIBUTES",
				"OTEL_TRACES_SAMPLER", "OTEL_TRACES_SAMPLER_ARG", "OTEL_SERVICE_NAME"} {
				t.Setenv(k, "")
				os.Unsetenv(k)
			}
			for i, kv := range tc.env {
				env[i] = [2]string{kv[0], strings.ReplaceAll(kv[1], "HOSTPORT", hostport)}
				t.Setenv(env[i][0], env[i][1])
			}
			o := exportOneSpan(t, c)
			got := lint(t, env)
			if tc.rule == "" {
				if len(got) != 0 {
					t.Errorf("healthy config: lostspans reported %+v", got)
				}
				if o.exporterErr != nil || len(o.delivered) != 1 {
					t.Errorf("healthy config: the SDK didn't deliver one span: err=%v delivered=%d", o.exporterErr, len(o.delivered))
				}
				return
			}
			found := false
			for _, f := range got {
				if f.Rule == tc.rule {
					found = true
					if f.Severity != tc.severity {
						t.Errorf("%s has severity %s, want %s", tc.rule, f.Severity, tc.severity)
					}
				}
			}
			if !found {
				t.Errorf("lostspans didn't report %s; got %+v", tc.rule, got)
			}
			if !tc.lost(o) {
				t.Errorf("the SDK didn't behave as %s says: exporterErr=%v resourceErr=%v delivered=%d", tc.rule, o.exporterErr, o.resourceErr, len(o.delivered))
			}
		})
	}
}
