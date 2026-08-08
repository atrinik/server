package observability

import (
	"math"
	"strings"
	"sync"
	"testing"
)

func TestMetricLabelsAreBoundedAndSnapshotStable(t *testing.T) {
	t.Parallel()
	registry := DefaultRegistry()
	if err := registry.AddCounter("atrinik_connections_total", 2, map[string]string{"result": "accepted"}); err != nil {
		t.Fatal(err)
	}
	if err := registry.AddCounter("atrinik_connections_total", 1, map[string]string{"result": "player-name"}); err == nil {
		t.Fatal("accepted high-cardinality label")
	}
	if err := registry.Observe("atrinik_tick_seconds", 0.004, nil); err != nil {
		t.Fatal(err)
	}
	first, err := registry.OpenMetrics(16_384)
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.OpenMetrics(16_384)
	if err != nil || string(first) != string(second) || !strings.HasSuffix(string(first), "# EOF\n") {
		t.Fatal("OpenMetrics snapshot is not deterministic")
	}
}

func TestDescriptorInputsAreCopiedAndHistogramBoundariesAreCumulative(t *testing.T) {
	t.Parallel()
	labels := map[string][]string{"result": {"accepted"}}
	buckets := []float64{1, 2}
	registry, err := NewRegistry([]Descriptor{{Name: "test_seconds", Help: "Test duration", Unit: "seconds", Kind: Histogram, Labels: labels, Buckets: buckets}})
	if err != nil {
		t.Fatal(err)
	}
	labels["result"][0] = "mutated"
	buckets[0] = 100
	if err := registry.Observe("test_seconds", 1, map[string]string{"result": "accepted"}); err != nil {
		t.Fatal("caller mutated registered descriptor:", err)
	}
	output, err := registry.OpenMetrics(4_096)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`le="1"} 1`, `le="2"} 1`, `le="+Inf"} 1`, "test_seconds_count{result=\"accepted\"} 1"} {
		if !strings.Contains(string(output), expected) {
			t.Fatalf("snapshot lacks %q:\n%s", expected, output)
		}
	}
}

func TestConcurrentSnapshotAndUpdatesRemainBounded(t *testing.T) {
	t.Parallel()
	registry := DefaultRegistry()
	var wait sync.WaitGroup
	for worker := range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range 1_000 {
				if err := registry.AddCounter("atrinik_connections_total", uint64(worker+1), map[string]string{"result": "accepted"}); err != nil {
					t.Errorf("update: %v", err)
					return
				}
				if _, err := registry.OpenMetrics(256 * 1024); err != nil {
					t.Errorf("snapshot: %v", err)
					return
				}
			}
		}()
	}
	wait.Wait()
}

func TestRejectsInjectedMetadataAndInvalidBuckets(t *testing.T) {
	t.Parallel()
	invalid := []Descriptor{
		{Name: "bad_total", Help: "line\nbreak", Unit: "items", Kind: Counter},
		{Name: "bad_total", Help: "bad buckets", Unit: "items", Kind: Counter, Buckets: []float64{1}},
		{Name: "bad_seconds", Help: "duplicates", Unit: "seconds", Kind: Histogram, Buckets: []float64{1, 1}},
	}
	for _, descriptor := range invalid {
		if _, err := NewRegistry([]Descriptor{descriptor}); err == nil {
			t.Fatalf("accepted invalid descriptor: %+v", descriptor)
		}
	}
}

func TestCounterSaturatesAndSnapshotBudgetFailsClosed(t *testing.T) {
	t.Parallel()
	registry, err := NewRegistry([]Descriptor{{Name: "test_total", Help: "test", Unit: "items", Kind: Counter}})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.AddCounter("test_total", math.MaxUint64, nil); err != nil {
		t.Fatal(err)
	}
	if err := registry.AddCounter("test_total", 1, nil); err != nil {
		t.Fatal(err)
	}
	output, err := registry.OpenMetrics(1_024)
	if err != nil || !strings.Contains(string(output), "18446744073709551615") {
		t.Fatal("counter did not saturate")
	}
	if _, err := registry.OpenMetrics(1); err == nil {
		t.Fatal("accepted an undersized snapshot budget")
	}
}
