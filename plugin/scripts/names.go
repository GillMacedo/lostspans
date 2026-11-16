package main

import "strings"

// knownNames are the SDK variables in the OpenTelemetry specification
// (configuration/sdk-environment-variables.md and protocol/exporter.md),
// plus the OTLP metrics exporter's two variables and the deprecated Jaeger ones.
var knownNames = func() map[string]bool {
	m := map[string]bool{}
	for _, n := range []string{
		"OTEL_SDK_DISABLED", "OTEL_RESOURCE_ATTRIBUTES", "OTEL_SERVICE_NAME", "OTEL_LOG_LEVEL",
		"OTEL_PROPAGATORS", "OTEL_TRACES_SAMPLER", "OTEL_TRACES_SAMPLER_ARG",
		"OTEL_BSP_SCHEDULE_DELAY", "OTEL_BSP_EXPORT_TIMEOUT", "OTEL_BSP_MAX_QUEUE_SIZE", "OTEL_BSP_MAX_EXPORT_BATCH_SIZE",
		"OTEL_BLRP_SCHEDULE_DELAY", "OTEL_BLRP_EXPORT_TIMEOUT", "OTEL_BLRP_MAX_QUEUE_SIZE", "OTEL_BLRP_MAX_EXPORT_BATCH_SIZE",
		"OTEL_ATTRIBUTE_VALUE_LENGTH_LIMIT", "OTEL_ATTRIBUTE_COUNT_LIMIT",
		"OTEL_SPAN_ATTRIBUTE_VALUE_LENGTH_LIMIT", "OTEL_SPAN_ATTRIBUTE_COUNT_LIMIT", "OTEL_SPAN_EVENT_COUNT_LIMIT",
		"OTEL_SPAN_LINK_COUNT_LIMIT", "OTEL_EVENT_ATTRIBUTE_COUNT_LIMIT", "OTEL_LINK_ATTRIBUTE_COUNT_LIMIT",
		"OTEL_LOGRECORD_ATTRIBUTE_VALUE_LENGTH_LIMIT", "OTEL_LOGRECORD_ATTRIBUTE_COUNT_LIMIT",
		"OTEL_EXPORTER_ZIPKIN_ENDPOINT", "OTEL_EXPORTER_ZIPKIN_TIMEOUT", "OTEL_EXPORTER_ZIPKIN_PROTOCOL",
		"OTEL_EXPORTER_PROMETHEUS_HOST", "OTEL_EXPORTER_PROMETHEUS_PORT",
		"OTEL_TRACES_EXPORTER", "OTEL_METRICS_EXPORTER", "OTEL_LOGS_EXPORTER",
		"OTEL_METRICS_EXEMPLAR_FILTER", "OTEL_METRIC_EXPORT_INTERVAL", "OTEL_METRIC_EXPORT_TIMEOUT", "OTEL_METRIC_EXPORT_MAX_BATCH_SIZE",
		"OTEL_CONFIG_FILE", "OTEL_EXPERIMENTAL_CONFIG_FILE",
		"OTEL_EXPORTER_OTLP_SPAN_INSECURE", "OTEL_EXPORTER_OTLP_METRIC_INSECURE",
		"OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE", "OTEL_EXPORTER_OTLP_METRICS_DEFAULT_HISTOGRAM_AGGREGATION",
		"OTEL_EXPORTER_JAEGER_AGENT_HOST", "OTEL_EXPORTER_JAEGER_AGENT_PORT", "OTEL_EXPORTER_JAEGER_ENDPOINT",
		"OTEL_EXPORTER_JAEGER_TIMEOUT", "OTEL_EXPORTER_JAEGER_USER", "OTEL_EXPORTER_JAEGER_PASSWORD",
		"OTEL_SEMCONV_STABILITY_OPT_IN", "OTEL_RESOURCE_DETECTORS",
	} {
		m[n] = true
	}
	for _, opt := range []string{"ENDPOINT", "INSECURE", "CERTIFICATE", "CLIENT_KEY", "CLIENT_CERTIFICATE", "HEADERS", "COMPRESSION", "TIMEOUT", "PROTOCOL"} {
		m["OTEL_EXPORTER_OTLP_"+opt] = true
		for _, sig := range []string{"TRACES", "METRICS", "LOGS"} {
			m["OTEL_EXPORTER_OTLP_"+sig+"_"+opt] = true
		}
	}
	return m
}()

// sdkPrefixes are language and project specific variables, following the
// spec's OTEL_{LANGUAGE}_{FEATURE} convention. Their names aren't checked.
var sdkPrefixes = []string{
	"OTEL_GO_", "OTEL_JAVA_", "OTEL_JAVAAGENT_", "OTEL_INSTRUMENTATION_", "OTEL_JMX_",
	"OTEL_PYTHON_", "OTEL_NODE_", "OTEL_NODEJS_", "OTEL_DOTNET_", "OTEL_PHP_", "OTEL_RUBY_",
	"OTEL_RUST_", "OTEL_CPP_", "OTEL_ERLANG_", "OTEL_ELIXIR_", "OTEL_SWIFT_", "OTEL_EXPERIMENTAL_",
	"OTEL_EBPF_", "OTEL_LAMBDA_", "OTEL_AWS_", "OTEL_GCP_", "OTEL_AZURE_",
}

func hasSDKPrefix(name string) bool {
	for _, p := range sdkPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// closestKnown returns a known name that name is probably a typo of, or "".
func closestKnown(name string) string {
	if knownNames[name] || hasSDKPrefix(name) {
		return ""
	}
	best, bestD := "", 3
	for k := range knownNames {
		d := editDistance(name, k)
		if d < bestD || (d == bestD && k < best) {
			best, bestD = k, d
		}
	}
	if bestD > 2 || len(name) < 12 {
		return ""
	}
	return best
}

func editDistance(a, b string) int {
	if a == b {
		return 0
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

// Kinds of value, from the spec's type of each variable.
var (
	booleanVars = map[string]bool{"OTEL_SDK_DISABLED": true, "OTEL_EXPORTER_OTLP_INSECURE": true,
		"OTEL_EXPORTER_OTLP_TRACES_INSECURE": true, "OTEL_EXPORTER_OTLP_METRICS_INSECURE": true,
		"OTEL_EXPORTER_OTLP_LOGS_INSECURE": true, "OTEL_EXPORTER_OTLP_SPAN_INSECURE": true,
		"OTEL_EXPORTER_OTLP_METRIC_INSECURE": true}
	timeoutVars = map[string]bool{"OTEL_EXPORTER_OTLP_TIMEOUT": true, "OTEL_EXPORTER_OTLP_TRACES_TIMEOUT": true,
		"OTEL_EXPORTER_OTLP_METRICS_TIMEOUT": true, "OTEL_EXPORTER_OTLP_LOGS_TIMEOUT": true,
		"OTEL_BSP_EXPORT_TIMEOUT": true, "OTEL_BLRP_EXPORT_TIMEOUT": true, "OTEL_METRIC_EXPORT_TIMEOUT": true,
		"OTEL_EXPORTER_ZIPKIN_TIMEOUT": true}
	durationVars = map[string]bool{"OTEL_BSP_SCHEDULE_DELAY": true, "OTEL_BLRP_SCHEDULE_DELAY": true,
		"OTEL_METRIC_EXPORT_INTERVAL": true}
	integerVars = map[string]bool{"OTEL_BSP_MAX_QUEUE_SIZE": true, "OTEL_BSP_MAX_EXPORT_BATCH_SIZE": true,
		"OTEL_BLRP_MAX_QUEUE_SIZE": true, "OTEL_BLRP_MAX_EXPORT_BATCH_SIZE": true,
		"OTEL_ATTRIBUTE_VALUE_LENGTH_LIMIT": true, "OTEL_ATTRIBUTE_COUNT_LIMIT": true,
		"OTEL_SPAN_ATTRIBUTE_VALUE_LENGTH_LIMIT": true, "OTEL_SPAN_ATTRIBUTE_COUNT_LIMIT": true,
		"OTEL_SPAN_EVENT_COUNT_LIMIT": true, "OTEL_SPAN_LINK_COUNT_LIMIT": true,
		"OTEL_EVENT_ATTRIBUTE_COUNT_LIMIT": true, "OTEL_LINK_ATTRIBUTE_COUNT_LIMIT": true,
		"OTEL_LOGRECORD_ATTRIBUTE_VALUE_LENGTH_LIMIT": true, "OTEL_LOGRECORD_ATTRIBUTE_COUNT_LIMIT": true,
		"OTEL_METRIC_EXPORT_MAX_BATCH_SIZE": true, "OTEL_EXPORTER_PROMETHEUS_PORT": true}
)

var (
	protocols  = []string{"grpc", "http/protobuf", "http/json"}
	samplers   = []string{"always_on", "always_off", "traceidratio", "parentbased_always_on", "parentbased_always_off", "parentbased_traceidratio", "parentbased_jaeger_remote", "jaeger_remote", "xray"}
	propagates = []string{"tracecontext", "baggage", "b3", "b3multi", "jaeger", "xray", "ottrace", "none"}
	exporters  = map[string][]string{
		"OTEL_TRACES_EXPORTER":  {"otlp", "zipkin", "console", "none", "otlp/stdout"},
		"OTEL_METRICS_EXPORTER": {"otlp", "prometheus", "console", "none", "otlp/stdout"},
		"OTEL_LOGS_EXPORTER":    {"otlp", "console", "none", "otlp/stdout"},
	}
	// sdkExporters are exporter names that some SDKs accept outside the spec.
	sdkExporters = []string{"otlp_proto_http", "otlp_proto_grpc", "logging-otlp", "experimental-otlp/stdout", "gcp_trace", "gcp_monitoring", "azure_monitor", "xray"}
)

func inList(v string, list []string) bool {
	for _, x := range list {
		if v == x {
			return true
		}
	}
	return false
}
