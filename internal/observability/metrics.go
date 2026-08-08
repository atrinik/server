package observability

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	maxMetricSeries  = 512
	maxMetricBuckets = 32
	maxMetricHelp    = 160
	maxMetricUnit    = 32
)

var metricName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// MetricKind defines the allowed metric operations.
type MetricKind string

const (
	Counter   MetricKind = "counter"
	Gauge     MetricKind = "gauge"
	Histogram MetricKind = "histogram"
)

// Descriptor registers stable name, unit, labels, and histogram bounds.
type Descriptor struct {
	Name    string
	Help    string
	Unit    string
	Kind    MetricKind
	Labels  map[string][]string
	Buckets []float64
}

type series struct {
	value   uint64
	buckets []uint64
	sum     float64
	count   uint64
}

// Registry stores bounded operational metrics. Gameplay never reads it.
type Registry struct {
	mu          sync.RWMutex
	descriptors map[string]Descriptor
	series      map[string]*series
}

// NewRegistry validates and registers a closed descriptor set.
func NewRegistry(descriptors []Descriptor) (*Registry, error) {
	registry := &Registry{descriptors: make(map[string]Descriptor, len(descriptors)), series: make(map[string]*series)}
	for _, descriptor := range descriptors {
		if len(descriptor.Name) > 120 || !metricName.MatchString(descriptor.Name) ||
			descriptor.Help == "" || len(descriptor.Help) > maxMetricHelp ||
			strings.ContainsAny(descriptor.Help, "\r\n\\") || descriptor.Unit == "" ||
			len(descriptor.Unit) > maxMetricUnit || !metricName.MatchString(descriptor.Unit) {
			return nil, errors.New("metric descriptor has invalid identity")
		}
		if _, duplicate := registry.descriptors[descriptor.Name]; duplicate {
			return nil, errors.New("duplicate metric descriptor")
		}
		if descriptor.Kind != Counter && descriptor.Kind != Gauge && descriptor.Kind != Histogram {
			return nil, errors.New("metric descriptor has invalid kind")
		}
		if descriptor.Kind != Histogram && len(descriptor.Buckets) != 0 {
			return nil, errors.New("only histograms may register buckets")
		}
		if len(descriptor.Labels) > 3 {
			return nil, errors.New("metric descriptor has too many labels")
		}
		labels := make(map[string][]string, len(descriptor.Labels))
		for name, values := range descriptor.Labels {
			if len(name) > maxFieldKey || !stableName.MatchString(name) || len(values) == 0 || len(values) > 16 {
				return nil, errors.New("metric label is unbounded")
			}
			for _, value := range values {
				if len(value) > maxFieldValue || !stableName.MatchString(value) {
					return nil, errors.New("metric label value is invalid")
				}
				if contains(labels[name], value) {
					return nil, errors.New("metric label value is duplicated")
				}
				labels[name] = append(labels[name], value)
			}
		}
		if len(descriptor.Buckets) > maxMetricBuckets {
			return nil, errors.New("histogram has too many buckets")
		}
		for index, bucket := range descriptor.Buckets {
			if math.IsNaN(bucket) || math.IsInf(bucket, 0) || bucket <= 0 || (index > 0 && bucket <= descriptor.Buckets[index-1]) {
				return nil, errors.New("histogram buckets must be finite, positive, and strictly increasing")
			}
		}
		descriptor.Labels = labels
		descriptor.Buckets = append([]float64(nil), descriptor.Buckets...)
		registry.descriptors[descriptor.Name] = descriptor
	}
	return registry, nil
}

// DefaultRegistry contains the complete M1 operational surface.
func DefaultRegistry() *Registry {
	descriptors := []Descriptor{
		{Name: "atrinik_connections_total", Help: "Accepted connections", Unit: "connections", Kind: Counter, Labels: map[string][]string{"result": {"accepted", "rejected"}}},
		{Name: "atrinik_authentication_total", Help: "Authentication outcomes", Unit: "attempts", Kind: Counter, Labels: map[string][]string{"result": {"accepted", "denied", "error"}}},
		{Name: "atrinik_packets_total", Help: "Validated protocol packets", Unit: "packets", Kind: Counter, Labels: map[string][]string{"direction": {"in", "out"}, "result": {"accepted", "rejected"}}},
		{Name: "atrinik_packet_bytes_total", Help: "Validated protocol packet bytes", Unit: "bytes", Kind: Counter, Labels: map[string][]string{"direction": {"in", "out"}}},
		{Name: "atrinik_protocol_rejects_total", Help: "Protocol rejection categories", Unit: "packets", Kind: Counter, Labels: map[string][]string{"reason": {"malformed", "unsupported", "unauthorized", "over-limit"}}},
		{Name: "atrinik_queue_drops_total", Help: "Rejected bounded queue submissions", Unit: "commands", Kind: Counter, Labels: map[string][]string{"queue": {"simulation", "storage", "telemetry"}}},
		{Name: "atrinik_map_loads_total", Help: "Map load outcomes", Unit: "maps", Kind: Counter, Labels: map[string][]string{"result": {"accepted", "rejected"}}},
		{Name: "atrinik_saves_total", Help: "Persistence outcomes", Unit: "transactions", Kind: Counter, Labels: map[string][]string{"result": {"committed", "rolled-back"}}},
		{Name: "atrinik_plugin_failures_total", Help: "Native extension failures", Unit: "failures", Kind: Counter, Labels: map[string][]string{"kind": {"compiled-content", "native-handler"}}},
		{Name: "atrinik_connected_clients", Help: "Current connected clients", Unit: "clients", Kind: Gauge},
		{Name: "atrinik_playing_clients", Help: "Current playing clients", Unit: "clients", Kind: Gauge},
		{Name: "atrinik_loaded_maps", Help: "Current loaded maps", Unit: "maps", Kind: Gauge},
		{Name: "atrinik_active_objects", Help: "Current active objects", Unit: "objects", Kind: Gauge},
		{Name: "atrinik_queue_depth", Help: "Current bounded queue depth", Unit: "commands", Kind: Gauge, Labels: map[string][]string{"queue": {"simulation", "storage", "telemetry"}}},
		{Name: "atrinik_queue_bytes", Help: "Current bounded queue bytes", Unit: "bytes", Kind: Gauge, Labels: map[string][]string{"queue": {"simulation", "storage", "telemetry"}}},
		{Name: "atrinik_memory_pool_bytes", Help: "Current bounded memory pool bytes", Unit: "bytes", Kind: Gauge, Labels: map[string][]string{"pool": {"packet", "object", "content"}}},
		{Name: "atrinik_tick_seconds", Help: "Simulation tick duration", Unit: "seconds", Kind: Histogram, Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1}},
		{Name: "atrinik_operation_seconds", Help: "Bounded operation duration", Unit: "seconds", Kind: Histogram, Labels: map[string][]string{"operation": {"packet", "map-load", "save", "content-handler", "quic-handshake"}}, Buckets: []float64{0.001, 0.01, 0.1, 1, 5}},
	}
	registry, err := NewRegistry(descriptors)
	if err != nil {
		panic(err)
	}
	return registry
}

// AddCounter adds delta and saturates rather than wrapping at uint64 overflow.
func (registry *Registry) AddCounter(name string, delta uint64, labels map[string]string) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	descriptor, key, err := registry.resolve(name, Counter, labels)
	if err != nil {
		return err
	}
	_ = descriptor
	metric, err := registry.getSeries(key, 0)
	if err != nil {
		return err
	}
	if math.MaxUint64-metric.value < delta {
		metric.value = math.MaxUint64
	} else {
		metric.value += delta
	}
	return nil
}

// SetGauge updates one bounded registered gauge.
func (registry *Registry) SetGauge(name string, value uint64, labels map[string]string) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	_, key, err := registry.resolve(name, Gauge, labels)
	if err != nil {
		return err
	}
	metric, err := registry.getSeries(key, 0)
	if err != nil {
		return err
	}
	metric.value = value
	return nil
}

// Observe records one finite non-negative duration/value.
func (registry *Registry) Observe(name string, value float64, labels map[string]string) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return errors.New("histogram observation must be finite and non-negative")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	descriptor, key, err := registry.resolve(name, Histogram, labels)
	if err != nil {
		return err
	}
	metric, err := registry.getSeries(key, len(descriptor.Buckets)+1)
	if err != nil {
		return err
	}
	for index, bound := range descriptor.Buckets {
		if value <= bound {
			metric.buckets[index] = saturatingAdd(metric.buckets[index], 1)
		}
	}
	metric.buckets[len(metric.buckets)-1] = saturatingAdd(metric.buckets[len(metric.buckets)-1], 1)
	if metric.sum > math.MaxFloat64-value {
		metric.sum = math.MaxFloat64
	} else {
		metric.sum += value
	}
	metric.count = saturatingAdd(metric.count, 1)
	return nil
}

func (registry *Registry) resolve(name string, kind MetricKind, labels map[string]string) (Descriptor, string, error) {
	descriptor, found := registry.descriptors[name]
	if !found || descriptor.Kind != kind {
		return Descriptor{}, "", errors.New("metric is not registered for this operation")
	}
	if len(labels) != len(descriptor.Labels) {
		return Descriptor{}, "", errors.New("metric labels differ from descriptor")
	}
	names := make([]string, 0, len(labels))
	for label, value := range labels {
		allowed, found := descriptor.Labels[label]
		if !found || !contains(allowed, value) {
			return Descriptor{}, "", errors.New("metric label value is not allowlisted")
		}
		names = append(names, label)
	}
	sort.Strings(names)
	parts := []string{name}
	for _, label := range names {
		parts = append(parts, label+"="+labels[label])
	}
	return descriptor, strings.Join(parts, "\x00"), nil
}

func (registry *Registry) getSeries(key string, bucketCount int) (*series, error) {
	if metric := registry.series[key]; metric != nil {
		return metric, nil
	}
	if len(registry.series) >= maxMetricSeries {
		return nil, errors.New("metric series limit reached")
	}
	metric := &series{buckets: make([]uint64, bucketCount)}
	registry.series[key] = metric
	return metric, nil
}

// OpenMetrics returns a stable bounded text snapshot for a sidecar/local exporter.
func (registry *Registry) OpenMetrics(maximumBytes int) ([]byte, error) {
	if maximumBytes < 1 || maximumBytes > 1_048_576 {
		return nil, errors.New("snapshot byte budget is outside supported bounds")
	}
	registry.mu.RLock()
	keys := make([]string, 0, len(registry.series))
	for key := range registry.series {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	descriptors := make(map[string]Descriptor, len(registry.descriptors))
	for name, descriptor := range registry.descriptors {
		descriptors[name] = descriptor
	}
	seriesSnapshot := make(map[string]series, len(keys))
	for _, key := range keys {
		current := registry.series[key]
		seriesSnapshot[key] = series{value: current.value, buckets: append([]uint64(nil), current.buckets...), sum: current.sum, count: current.count}
	}
	registry.mu.RUnlock()

	var output strings.Builder
	emittedMetadata := make(map[string]struct{})
	for _, key := range keys {
		parts := strings.Split(key, "\x00")
		name := parts[0]
		metric := seriesSnapshot[key]
		descriptor := descriptors[name]
		if _, emitted := emittedMetadata[name]; !emitted {
			fmt.Fprintf(&output, "# HELP %s %s\n# TYPE %s %s\n# UNIT %s %s\n", name, descriptor.Help, name, descriptor.Kind, name, descriptor.Unit)
			emittedMetadata[name] = struct{}{}
		}
		labels := formatLabels(parts[1:])
		if descriptor.Kind == Histogram {
			for index, count := range metric.buckets {
				bound := "+Inf"
				if index < len(descriptor.Buckets) {
					bound = strconv.FormatFloat(descriptor.Buckets[index], 'g', -1, 64)
				}
				fmt.Fprintf(&output, "%s_bucket%s %d\n", name, appendLabel(labels, "le", bound), count)
			}
			fmt.Fprintf(&output, "%s_sum%s %g\n%s_count%s %d\n", name, labels, metric.sum, name, labels, metric.count)
		} else {
			fmt.Fprintf(&output, "%s%s %d\n", name, labels, metric.value)
		}
		if output.Len() > maximumBytes {
			return nil, errors.New("metric snapshot exceeds byte budget")
		}
	}
	output.WriteString("# EOF\n")
	if output.Len() > maximumBytes {
		return nil, errors.New("metric snapshot exceeds byte budget")
	}
	return []byte(output.String()), nil
}

func saturatingAdd(value, delta uint64) uint64 {
	if math.MaxUint64-value < delta {
		return math.MaxUint64
	}
	return value + delta
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func formatLabels(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	formatted := make([]string, 0, len(parts))
	for _, part := range parts {
		name, value, _ := strings.Cut(part, "=")
		formatted = append(formatted, name+`="`+value+`"`)
	}
	return "{" + strings.Join(formatted, ",") + "}"
}

func appendLabel(existing, name, value string) string {
	label := name + `="` + value + `"`
	if existing == "" {
		return "{" + label + "}"
	}
	return strings.TrimSuffix(existing, "}") + "," + label + "}"
}
