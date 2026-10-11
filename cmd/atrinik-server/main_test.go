package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/atrinik/server/internal/buildinfo"
	"github.com/atrinik/server/internal/config"
)

func TestVersionAndRedactedConfigCommands(t *testing.T) {
	t.Setenv("ATRINIK_ADMIN_TOKEN", strings.Repeat("s", 32))
	var output bytes.Buffer
	if err := run([]string{"version"}, &output, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var version buildinfo.Info
	if err := json.Unmarshal(output.Bytes(), &version); err != nil {
		t.Fatal(err)
	}
	if version.Version == "" || version.Revision == "" || version.GoVersion == "" || version.Target == "" || version.ProtocolContract == "" || version.ContentFormatRange == "" {
		t.Fatalf("incomplete version response: %+v", version)
	}

	output.Reset()
	if err := run([]string{"config", "-queue-capacity=64", "-commands-per-tick=16"}, &output, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), strings.Repeat("s", 32)) {
		t.Fatal("config command exposed diagnostics token")
	}
	var settings config.RedactedConfig
	if err := json.Unmarshal(output.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.AdminToken != "[redacted]" || settings.QueueCapacity != 64 || settings.CommandsPerTick != 16 {
		t.Fatalf("unexpected redacted config: %+v", settings)
	}
}

func TestCommandErrorsAreExplicit(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{nil, {"unknown"}, {"config", "extra"}, {"config", "-queue-capacity=0"}} {
		if err := run(arguments, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatalf("arguments %q unexpectedly succeeded", arguments)
		}
	}
}

func TestAccessPolicyIsIndependentOfPublicationVisibility(t *testing.T) {
	t.Parallel()
	for _, public := range []bool{false, true} {
		for _, required := range []bool{false, true} {
			var output bytes.Buffer
			args := []string{"config", "-server-public=" + strconv.FormatBool(public), "-access-required=" + strconv.FormatBool(required)}
			if err := run(args, &output, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			var settings config.RedactedConfig
			if err := json.Unmarshal(output.Bytes(), &settings); err != nil {
				t.Fatal(err)
			}
			if settings.Publisher.Public != public || settings.Publisher.AccessRequired != required {
				t.Fatal("configuration conflated visibility and admission policy")
			}
			if strings.Contains(output.String(), "password_required") || !strings.Contains(output.String(), "access_required") {
				t.Fatal("configuration does not use the access-token policy field")
			}
			configuration := config.Default().Publisher
			configuration.Public, configuration.AccessRequired = public, required
			snapshot, err := configuredSnapshot(configuration)
			if err != nil || snapshot.Public != public || snapshot.AccessRequired != required || snapshot.ProtocolMinor != 1 {
				t.Fatal("publication snapshot lost admission policy")
			}
		}
	}
	for _, minor := range []string{"0", "2"} {
		if err := run([]string{"config", "-publish-origin=https://publish.meta.atrinik.org", "-protocol-minor=" + minor}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatal("unsupported publisher protocol version was accepted")
		}
	}
	if err := run([]string{"config", "-password-required=true"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("obsolete shared-password flag was accepted")
	}
}
