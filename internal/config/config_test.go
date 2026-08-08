package config

import (
	"strings"
	"testing"
)

func TestRedactedNeverContainsToken(t *testing.T) {
	t.Parallel()
	configuration := Default()
	configuration.AdminToken = "top-secret-token"
	if got := configuration.Redacted().AdminToken; got != "[redacted]" {
		t.Fatalf("redacted token = %q", got)
	}
	if strings.Contains(configuration.Redacted().AdminToken, configuration.AdminToken) {
		t.Fatal("redacted configuration contains token")
	}
}

func TestRejectsTraversalAndUnboundedWork(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"../state", "state/../other", "/tmp/state"} {
		configuration := Default()
		configuration.StateDirectory = path
		if err := configuration.Validate(); err == nil {
			t.Fatalf("accepted unsafe state path %q", path)
		}
	}
	configuration := Default()
	configuration.CommandsPerTick = configuration.QueueCapacity + 1
	if err := configuration.Validate(); err == nil {
		t.Fatal("accepted commands-per-tick above queue capacity")
	}
	configuration = Default()
	configuration.AdminToken = "weak"
	if err := configuration.Validate(); err == nil {
		t.Fatal("accepted weak diagnostics token")
	}
}
