// Copyright 2026 The Atrinik Project
// SPDX-License-Identifier: MIT

package publisher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	metaserverv1 "github.com/atrinik/protocol/gen/go/atrinik/metaserver/v1"
	protocolmeta "github.com/atrinik/protocol/metaserver"
)

const fixtureCertificateBase64 = "MIIBlDCCATqgAwIBAgIBAjAKBggqhkjOPQQDAjApMScwJQYDVQQDDB5BdHJpbmlrIGdhbWUgcHVibGlzaGVyIGZpeHR1cmUwHhcNMjYwODEwMDI0MjMwWhcNMzYwODA3MDI0MjMwWjApMScwJQYDVQQDDB5BdHJpbmlrIGdhbWUgcHVibGlzaGVyIGZpeHR1cmUwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARp3V9S6hwQtev297vKo09IIjxFJ03bkJGWhINrtl02+qX74Y1fqMEglkyDsDS5uaw9wkEqAZFjvqGds9Nlh8mLo1MwUTAdBgNVHQ4EFgQUM2ynnFKEB/m1Ih384LbZpnCE6KcwHwYDVR0jBBgwFoAUM2ynnFKEB/m1Ih384LbZpnCE6KcwDwYDVR0TAQH/BAUwAwEB/zAKBggqhkjOPQQDAgNIADBFAiA+GS9rOiluma03pBE7eOsp8qQWF2x5LLzwIvHtOr9cQQIhALFjaapew4tGe7YyjGNwCqd7ga08+HeUd0L2+KBkaORJ"

const fixtureBody = "{\"schema\":\"atrinik-game-publish-v1\",\"serverId\":\"0145f46149b8483d33b8e02c9495b3e4ff2dd5ce342a22bb40913bba7a457d39\",\"certificate\":\"" + fixtureCertificateBase64 + "\",\"name\":\"Atrinik Game Alpha\",\"description\":\"Cooperative Ω\",\"region\":\"eu-west\",\"protocol\":{\"major\":1,\"minor\":0},\"content\":{\"id\":\"atrinik-main\",\"revisionSha256\":\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"},\"players\":{\"online\":3,\"capacity\":64},\"status\":\"online\",\"public\":true,\"passwordRequired\":false,\"endpoint\":{\"hostname\":\"xn--bcher-kva.example.org\",\"port\":13327}}"

func TestProtocolGoldenVectorBuildsAndVerifies(t *testing.T) {
	t.Parallel()
	certificate, err := base64.StdEncoding.DecodeString(fixtureCertificateBase64)
	if err != nil {
		t.Fatal(err)
	}
	var nonce [16]byte
	for index := range nonce {
		nonce[index] = byte(index)
	}
	components, err := protocolmeta.Build(protocolmeta.Parameters{
		Profile: protocolmeta.GameProfile, Authority: "publish.meta.atrinik.org",
		ServerID: "0145f46149b8483d33b8e02c9495b3e4ff2dd5ce342a22bb40913bba7a457d39",
		Sequence: 42, Nonce: nonce, Created: 1_800_000_000,
	}, []byte(fixtureBody))
	if err != nil {
		t.Fatal(err)
	}
	if components.ContentDigest != "sha-256=:Fzvnb28jWhv3lSDxHJKDzlJjNLnrc+1I+VLVdrqERGE=:" ||
		components.Path != "/v1/servers/0145f46149b8483d33b8e02c9495b3e4ff2dd5ce342a22bb40913bba7a457d39/publish" {
		t.Fatalf("golden components diverged: %+v", components)
	}
	signature, err := base64.StdEncoding.DecodeString("PcGj3kxhPCqdxaEAUNkpVaj2Xu+b+3LkfIgu/pD8M+94GlUu/0EbjDVGgn41yqkzyLQZBtI6c3DzL/JzTG/QaQ==")
	if err != nil || protocolmeta.VerifyCertificateSignature(certificate, "0145f46149b8483d33b8e02c9495b3e4ff2dd5ce342a22bb40913bba7a457d39", components.SignatureBase, signature) != nil {
		t.Fatal("protocol golden signature did not verify")
	}
	parsed, err := protocolmeta.ParseGamePublishJSON([]byte(fixtureBody))
	if err != nil || parsed.Server.Name != "Atrinik Game Alpha" {
		t.Fatalf("golden body did not parse: %v", err)
	}
}

func TestClientSendsExactSignedRequestAndConsumesAmbiguousSequences(t *testing.T) {
	t.Parallel()
	identity := testIdentity(t)
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	sequence, err := OpenSequenceStore(root, "sequence.log")
	if err != nil {
		t.Fatal(err)
	}
	defer sequence.Close()
	requests := make(chan *http.Request, 2)
	responses := 0
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		requests <- request.Clone(context.Background())
		responses++
		if responses == 1 {
			return nil, io.ErrUnexpectedEOF
		}
		return publisherResponse(http.StatusOK, `{"status":"ok","rendezvousToken":"`+strings.Repeat("a", 64)+`"}`, "application/json", nil), nil
	})
	client, err := NewClient("https://publish.meta.atrinik.org", identity, sequence, transport)
	if err != nil {
		t.Fatal(err)
	}
	client.entropy = bytes.NewReader(append(bytes.Repeat([]byte{1}, 16), bytes.Repeat([]byte{2}, 16)...))
	client.now = func() time.Time { return time.Unix(1_800_000_000, 0) }
	snapshot := testSnapshot()
	first, err := client.Publish(context.Background(), snapshot)
	if err != nil || first.Kind != ResultTransient {
		t.Fatalf("first result = %+v, %v", first, err)
	}
	second, err := client.Publish(context.Background(), snapshot)
	if err != nil || second.Kind != ResultAccepted || second.RendezvousToken[0] != 0xaa {
		t.Fatalf("second result = %+v, %v", second, err)
	}
	for expectedSequence := 1; expectedSequence <= 2; expectedSequence++ {
		request := <-requests
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if request.Method != http.MethodPost || request.URL.Host != "publish.meta.atrinik.org" ||
			request.Header.Get("Atrinik-Publish-Sequence") != strconv.Itoa(expectedSequence) ||
			request.Header.Get("Content-Type") != protocolmeta.ContentType {
			t.Fatalf("request %d is not canonical: %s %s %#v", expectedSequence, request.Method, request.URL, request.Header)
		}
		parsed, err := protocolmeta.ParseGamePublishJSON(body)
		if err != nil || !bytes.Equal(parsed.Server.ServerId, mustDecodeHex(t, identity.serverID)) || parsed.Server.Endpoint != nil {
			t.Fatalf("request body is invalid: %v", err)
		}
		nonce := [16]byte{}
		for index := range nonce {
			nonce[index] = byte(expectedSequence)
		}
		components, err := protocolmeta.Build(protocolmeta.Parameters{
			Profile: protocolmeta.GameProfile, Authority: request.URL.Host,
			ServerID: identity.serverID, Sequence: uint64(expectedSequence), Nonce: nonce,
			Created: 1_800_000_000,
		}, body)
		if err != nil || request.Header.Get("Content-Digest") != components.ContentDigest ||
			request.Header.Get("Signature-Input") != components.SignatureInput {
			t.Fatalf("signed fields diverged: %v", err)
		}
		signatureField := request.Header.Get("Signature")
		signature, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(strings.TrimPrefix(signatureField, "atrinik=:"), ":"))
		if err != nil || protocolmeta.VerifyCertificateSignature(identity.certificateDER, identity.serverID, components.SignatureBase, signature) != nil {
			t.Fatal("request signature did not verify")
		}
	}
	if sequence.HighWater() != 2 {
		t.Fatalf("high water = %d", sequence.HighWater())
	}
}

func TestClientPublishesOnlyAnExplicitCanonicalEndpoint(t *testing.T) {
	t.Parallel()
	identity := testIdentity(t)
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	sequence, err := OpenSequenceStore(root, "sequence.log")
	if err != nil {
		t.Fatal(err)
	}
	defer sequence.Close()
	client, err := NewClient("https://publish.meta.atrinik.org", identity, sequence, roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF
	}))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := testSnapshot()
	snapshot.Public = false
	snapshot.Endpoint = &metaserverv1.DirectEndpoint{Hostname: "play.example.net", Port: 13327}
	body, err := client.bodyFor(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := protocolmeta.ParseGamePublishJSON(body)
	if err != nil || parsed.Public || parsed.Server.Endpoint == nil ||
		parsed.Server.Endpoint.Hostname != "play.example.net" || parsed.Server.Endpoint.Port != 13327 {
		t.Fatalf("explicit endpoint body = %#v, %v", parsed, err)
	}
	snapshot.Endpoint.Hostname = "192.0.2.1"
	if err := client.ValidateSnapshot(snapshot); err == nil {
		t.Fatal("numeric endpoint was accepted")
	}
}

func TestClientResponsePolicyIsBoundedAndReplayRaisesSequence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		status    int
		body      string
		content   string
		headers   http.Header
		want      ResultKind
		wantRetry time.Duration
		wantHigh  uint64
	}{
		{"replay", 409, `{"error":{"code":"publish_replay","minimumNextSequence":"9"}}`, "application/json", nil, ResultReplay, 0, 8},
		{"rate", 429, `{"error":{"code":"rate_limited","message":"The request budget has been exhausted.","reason":"publish_daily","retry_after_seconds":60}}`, "application/json; charset=utf-8", http.Header{"Retry-After": {"60"}}, ResultRateLimited, time.Minute, 1},
		{"bad retry", 429, `{}`, "application/json; charset=utf-8", http.Header{"Retry-After": {"60"}}, ResultPermanent, 0, 1},
		{"server failure", 503, `{}`, "application/json; charset=utf-8", nil, ResultTransient, 0, 1},
		{"authentication", 401, `{}`, "application/json; charset=utf-8", nil, ResultPermanent, 0, 1},
		{"oversized success", 200, strings.Repeat("x", maximumResponseBytes+1), "application/json", nil, ResultPermanent, 0, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			identity := testIdentity(t)
			root, err := os.OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			sequence, err := OpenSequenceStore(root, "sequence.log")
			if err != nil {
				t.Fatal(err)
			}
			defer sequence.Close()
			transport := roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return publisherResponse(test.status, test.body, test.content, test.headers), nil
			})
			client, err := NewClient("https://publish.meta.atrinik.org", identity, sequence, transport)
			if err != nil {
				t.Fatal(err)
			}
			client.entropy = bytes.NewReader(bytes.Repeat([]byte{1}, 16))
			client.now = func() time.Time { return time.Unix(1_800_000_000, 0) }
			result, err := client.Publish(context.Background(), testSnapshot())
			if err != nil || result.Kind != test.want || result.RetryAfter != test.wantRetry || sequence.HighWater() != test.wantHigh {
				t.Fatalf("result = %+v, error = %v, high = %d", result, err, sequence.HighWater())
			}
		})
	}
}

func TestClientRetriesAnIncompleteSuccessResponse(t *testing.T) {
	t.Parallel()
	identity := testIdentity(t)
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	sequence, err := OpenSequenceStore(root, "sequence.log")
	if err != nil {
		t.Fatal(err)
	}
	defer sequence.Close()
	transport := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		response := publisherResponse(http.StatusOK, "", "application/json", nil)
		response.Body = io.NopCloser(errorReader{})
		return response, nil
	})
	client, err := NewClient("https://publish.meta.atrinik.org", identity, sequence, transport)
	if err != nil {
		t.Fatal(err)
	}
	client.entropy = bytes.NewReader(bytes.Repeat([]byte{1}, 16))
	client.now = func() time.Time { return time.Unix(1_800_000_000, 0) }
	result, err := client.Publish(context.Background(), testSnapshot())
	if err != nil || result.Kind != ResultTransient {
		t.Fatalf("incomplete response = %+v, %v", result, err)
	}
}

func TestClientGivesUsablePermanentStatusPrecedenceOverBodyFailure(t *testing.T) {
	t.Parallel()
	identity := testIdentity(t)
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	sequence, err := OpenSequenceStore(root, "sequence.log")
	if err != nil {
		t.Fatal(err)
	}
	defer sequence.Close()
	transport := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		response := publisherResponse(http.StatusUnauthorized, "", "application/json; charset=utf-8", nil)
		response.Body = io.NopCloser(errorReader{})
		return response, nil
	})
	client, err := NewClient("https://publish.meta.atrinik.org", identity, sequence, transport)
	if err != nil {
		t.Fatal(err)
	}
	client.entropy = bytes.NewReader(bytes.Repeat([]byte{1}, 16))
	client.now = func() time.Time { return time.Unix(1_800_000_000, 0) }
	result, err := client.Publish(context.Background(), testSnapshot())
	if err != nil || result.Kind != ResultPermanent {
		t.Fatalf("authentication response = %+v, %v", result, err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func publisherResponse(status int, body, contentType string, extra http.Header) *http.Response {
	headers := http.Header{
		"Cache-Control":          {"no-store"},
		"Content-Type":           {contentType},
		"X-Content-Type-Options": {"nosniff"},
	}
	for key, values := range extra {
		headers[key] = append([]string(nil), values...)
	}
	return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body))}
}

func testIdentity(t *testing.T) *Identity {
	t.Helper()
	certificate, key := testIdentityPEM(t)
	identity, err := ParseIdentityPEM(certificate, key)
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

func testSnapshot() Snapshot {
	digest := sha256.Sum256([]byte("content"))
	return Snapshot{
		Name: "Test Server", Description: "", ProtocolMinor: 0, ContentID: "atrinik-main",
		ContentRevisionSHA256: digest, PlayersOnline: 1, PlayersCapacity: 10,
		Status: metaserverv1.DirectoryServerStatus_DIRECTORY_SERVER_STATUS_ONLINE,
		Public: true,
	}
}

func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}
