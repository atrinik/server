package config

import (
	"strings"
	"testing"
	"time"
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

func TestPublisherConfigurationIsDisabledByDefaultAndStrictWhenEnabled(t *testing.T) {
	t.Parallel()
	configuration := Default()
	if configuration.Publisher.Enabled() || configuration.Validate() != nil {
		t.Fatal("default publisher configuration is not safely disabled")
	}
	configuration.Publisher.Origin = "https://publish.meta.atrinik.org"
	configuration.Publisher.Public = true
	if err := configuration.Validate(); err != nil {
		t.Fatalf("valid publisher configuration failed: %v", err)
	}
	for _, mutate := range []func(*PublisherConfig){
		func(value *PublisherConfig) { value.Origin = "http://publish.meta.atrinik.org" },
		func(value *PublisherConfig) { value.Origin = "https://publish.meta.atrinik.org/path" },
		func(value *PublisherConfig) { value.CertificatePath = "../certificate.pem" },
		func(value *PublisherConfig) { value.Name = "bad\nname" },
		func(value *PublisherConfig) { value.ContentRevisionSHA256 = strings.Repeat("A", 64) },
		func(value *PublisherConfig) { value.DirectHostname = "192.0.2.1"; value.DirectPort = 13327 },
		func(value *PublisherConfig) { value.DirectHostname = "play.example.net"; value.DirectPort = 0 },
		func(value *PublisherConfig) { value.PlayersCapacity = 100_001 },
		func(value *PublisherConfig) { value.HeartbeatInterval = 30 * time.Minute },
		func(value *PublisherConfig) { value.HeartbeatInterval = 211 * time.Minute },
	} {
		invalid := configuration
		mutate(&invalid.Publisher)
		if err := invalid.Validate(); err == nil {
			t.Fatalf("invalid publisher configuration passed: %+v", invalid.Publisher.Redacted())
		}
	}
}

func TestRejectsTraversalAndUnboundedWork(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"../state", "state/../other", "/tmp/state", `C:\state`, `\\server\state`} {
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
