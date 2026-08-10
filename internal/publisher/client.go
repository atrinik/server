// Copyright 2026 The Atrinik Project
// SPDX-License-Identifier: MIT

package publisher

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	metaserverv1 "github.com/atrinik/protocol/gen/go/atrinik/metaserver/v1"
	protocolmeta "github.com/atrinik/protocol/metaserver"
)

const (
	maximumResponseBytes       = 2 * 1024
	maximumResponseHeaderBytes = 16 * 1024
	maximumRetryAfter          = 24 * time.Hour
	defaultRequestTimeout      = 30 * time.Second
)

var (
	successBodyPattern  = regexp.MustCompile(`^\{"status":"ok","rendezvousToken":"([0-9a-f]{64})"\}$`)
	replayBodyPattern   = regexp.MustCompile(`^\{"error":\{"code":"publish_replay","minimumNextSequence":"([1-9][0-9]{0,19})"\}\}$`)
	errResponseTooLarge = errors.New("publisher response is outside supported bounds")
)

// Snapshot is one immutable public/private publication input. Identity fields
// are supplied only by the loaded certificate and cannot be overridden here.
type Snapshot struct {
	Name                  string
	Description           string
	Region                *string
	ProtocolMinor         uint32
	ContentID             string
	ContentRevisionSHA256 [32]byte
	PlayersOnline         uint32
	PlayersCapacity       uint32
	Status                metaserverv1.DirectoryServerStatus
	Public                bool
	PasswordRequired      bool
	Endpoint              *metaserverv1.DirectEndpoint
}

// ResultKind is a closed scheduling decision. It contains no response data.
type ResultKind string

const (
	ResultAccepted    ResultKind = "accepted"
	ResultReplay      ResultKind = "replay"
	ResultRateLimited ResultKind = "rate-limited"
	ResultTransient   ResultKind = "transient"
	ResultPermanent   ResultKind = "permanent"
)

// Result is one bounded attempt outcome. RendezvousToken must be consumed or
// discarded immediately and never persisted by the publisher.
type Result struct {
	Kind                ResultKind
	RetryAfter          time.Duration
	MinimumNextSequence uint64
	RendezvousToken     [32]byte
}

// Client constructs and sends exact signed publisher requests.
type Client struct {
	authority string
	origin    string
	identity  *Identity
	sequence  *SequenceStore
	http      *http.Client
	entropy   io.Reader
	now       func() time.Time
}

// NewClient validates an HTTPS origin and constructs a redirect-free client.
// A nil transport uses Go's verified system-root HTTPS transport.
func NewClient(origin string, identity *Identity, sequence *SequenceStore, transport http.RoundTripper) (*Client, error) {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Host != parsed.Hostname() ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.EscapedPath() != "" && parsed.EscapedPath() != "/") ||
		origin != strings.ToLower(origin) || identity == nil || sequence == nil {
		return nil, errors.New("publisher origin or identity is invalid")
	}
	if transport == nil {
		defaultTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, errors.New("publisher default transport is unavailable")
		}
		boundedTransport := defaultTransport.Clone()
		boundedTransport.MaxResponseHeaderBytes = maximumResponseHeaderBytes
		transport = boundedTransport
	}
	var validationNonce [16]byte
	validationNonce[0] = 1
	if _, err := protocolmeta.Build(protocolmeta.Parameters{
		Profile: protocolmeta.GameProfile, Authority: parsed.Hostname(),
		ServerID: identity.serverID, Sequence: 1, Nonce: validationNonce, Created: 1,
	}, []byte{'{'}); err != nil {
		return nil, errors.New("publisher origin or identity is invalid")
	}
	return &Client{
		authority: parsed.Hostname(),
		origin:    "https://" + parsed.Hostname(),
		identity:  identity,
		sequence:  sequence,
		http: &http.Client{
			Transport: transport,
			Timeout:   defaultRequestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errors.New("publisher redirects are forbidden")
			},
		},
		entropy: rand.Reader,
		now:     time.Now,
	}, nil
}

// Publish consumes one sequence and nonce even when the result is ambiguous.
func (client *Client) Publish(ctx context.Context, snapshot Snapshot) (Result, error) {
	request, localRetryAfter, err := client.buildRequest(ctx, snapshot)
	if err != nil {
		return Result{}, err
	}
	if localRetryAfter > 0 {
		return Result{Kind: ResultRateLimited, RetryAfter: localRetryAfter}, nil
	}
	response, err := client.http.Do(request)
	if err != nil {
		return Result{Kind: ResultTransient}, nil
	}
	defer response.Body.Close()
	body, err := readBoundedResponse(response.Body)
	if err != nil {
		if response.StatusCode == http.StatusRequestTimeout || response.StatusCode >= 500 {
			return Result{Kind: ResultTransient, RetryAfter: boundedResponseRetryAfter(response.Header)}, nil
		}
		if !errors.Is(err, errResponseTooLarge) &&
			(response.StatusCode == http.StatusOK || response.StatusCode == http.StatusConflict || response.StatusCode == http.StatusTooManyRequests) {
			return Result{Kind: ResultTransient}, nil
		}
		return Result{Kind: ResultPermanent}, nil
	}
	defer clear(body)
	return client.classifyResponse(response, body)
}

// ValidateSnapshot applies the exact protocol-owned body contract without
// reserving a sequence or touching the network.
func (client *Client) ValidateSnapshot(snapshot Snapshot) error {
	_, err := client.bodyFor(snapshot)
	return err
}

func (client *Client) buildRequest(ctx context.Context, snapshot Snapshot) (*http.Request, time.Duration, error) {
	body, err := client.bodyFor(snapshot)
	if err != nil {
		return nil, 0, err
	}
	createdAt := client.now().UTC()
	sequence, localRetryAfter, err := client.sequence.Reserve(createdAt)
	if err != nil || localRetryAfter > 0 {
		return nil, localRetryAfter, err
	}
	var nonce [16]byte
	if _, err := io.ReadFull(client.entropy, nonce[:]); err != nil {
		return nil, 0, errors.New("generate publisher nonce")
	}
	if bytes.Equal(nonce[:], make([]byte, len(nonce))) {
		return nil, 0, errors.New("generate publisher nonce")
	}
	created := createdAt.Unix()
	components, err := protocolmeta.Build(protocolmeta.Parameters{
		Profile: protocolmeta.GameProfile, Authority: client.authority,
		ServerID: client.identity.serverID, Sequence: sequence,
		Nonce: nonce, Created: created,
	}, body)
	if err != nil {
		return nil, 0, errors.New("build publisher signature components")
	}
	signature, err := protocolmeta.Sign(client.identity.privateKey, components.SignatureBase)
	if err != nil {
		return nil, 0, errors.New("sign publisher request")
	}
	encodedSignature := base64.StdEncoding.EncodeToString(signature)
	clear(signature)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.origin+components.Path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, errors.New("build publisher request")
	}
	request.Header.Set("Content-Type", protocolmeta.ContentType)
	request.Header.Set("Content-Digest", components.ContentDigest)
	request.Header.Set("Atrinik-Server-ID", client.identity.serverID)
	request.Header.Set("Atrinik-Publish-Sequence", strconv.FormatUint(sequence, 10))
	request.Header.Set("Signature-Input", components.SignatureInput)
	request.Header.Set("Signature", protocolmeta.SignatureLabel+"=:"+encodedSignature+":")
	return request, 0, nil
}

func (client *Client) bodyFor(snapshot Snapshot) ([]byte, error) {
	serverID, err := hex.DecodeString(client.identity.serverID)
	if err != nil {
		return nil, errors.New("publisher identity is invalid")
	}
	server := &metaserverv1.DirectoryServer{
		ServerId:              serverID,
		CertificateSha256:     append([]byte(nil), serverID...),
		Name:                  snapshot.Name,
		Description:           snapshot.Description,
		Region:                cloneString(snapshot.Region),
		ProtocolMajor:         1,
		ProtocolMinor:         snapshot.ProtocolMinor,
		ContentId:             snapshot.ContentID,
		ContentRevisionSha256: append([]byte(nil), snapshot.ContentRevisionSHA256[:]...),
		PlayersOnline:         snapshot.PlayersOnline,
		PlayersCapacity:       snapshot.PlayersCapacity,
		Status:                snapshot.Status,
		PasswordRequired:      snapshot.PasswordRequired,
		Endpoint:              cloneEndpoint(snapshot.Endpoint),
	}
	body, err := protocolmeta.MarshalGamePublishJSON(&protocolmeta.GamePublishRequest{
		CertificateDER: client.identity.certificateDER,
		Server:         server,
		Public:         snapshot.Public,
	})
	if err != nil {
		return nil, errors.New("publisher snapshot is invalid")
	}
	return body, nil
}

func (client *Client) classifyResponse(response *http.Response, body []byte) (Result, error) {
	switch response.StatusCode {
	case http.StatusOK:
		if !validResponseHeaders(response.Header, "application/json") {
			return Result{Kind: ResultPermanent}, nil
		}
		match := successBodyPattern.FindSubmatch(body)
		if match == nil {
			return Result{Kind: ResultPermanent}, nil
		}
		decoded, _ := hex.DecodeString(string(match[1]))
		var token [32]byte
		copy(token[:], decoded)
		return Result{Kind: ResultAccepted, RendezvousToken: token}, nil
	case http.StatusConflict:
		if !validResponseHeaders(response.Header, "application/json") {
			return Result{Kind: ResultPermanent}, nil
		}
		match := replayBodyPattern.FindSubmatch(body)
		if match == nil {
			return Result{Kind: ResultPermanent}, nil
		}
		minimum, err := strconv.ParseUint(string(match[1]), 10, 64)
		if err != nil || minimum == 0 {
			return Result{Kind: ResultPermanent}, nil
		}
		if err := client.sequence.AdvanceMinimum(minimum); err != nil {
			return Result{}, err
		}
		return Result{Kind: ResultReplay, MinimumNextSequence: minimum}, nil
	case http.StatusTooManyRequests:
		return classifyRateLimit(response, body), nil
	case http.StatusRequestTimeout:
		return Result{Kind: ResultTransient}, nil
	default:
		if response.StatusCode >= 500 && response.StatusCode <= 599 {
			return Result{Kind: ResultTransient, RetryAfter: boundedResponseRetryAfter(response.Header)}, nil
		}
		return Result{Kind: ResultPermanent}, nil
	}
}

func boundedResponseRetryAfter(header http.Header) time.Duration {
	value := header.Get("Retry-After")
	if value == "" {
		return 0
	}
	seconds, err := strconv.ParseUint(value, 10, 32)
	if err != nil || seconds < 1 || seconds > uint64(maximumRetryAfter/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func classifyRateLimit(response *http.Response, body []byte) Result {
	if !validResponseHeaders(response.Header, "application/json; charset=utf-8") {
		return Result{Kind: ResultPermanent}
	}
	retrySeconds, err := strconv.ParseUint(response.Header.Get("Retry-After"), 10, 32)
	if err != nil || retrySeconds < 1 || retrySeconds > uint64(maximumRetryAfter/time.Second) {
		return Result{Kind: ResultPermanent}
	}
	var envelope struct {
		Error struct {
			Code              string `json:"code"`
			Message           string `json:"message"`
			Reason            string `json:"reason"`
			RetryAfterSeconds uint64 `json:"retry_after_seconds"`
		} `json:"error"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		envelope.Error.Code != "rate_limited" ||
		envelope.Error.Message != "The request budget has been exhausted." ||
		(envelope.Error.Reason != "global_burst" && envelope.Error.Reason != "publish_burst" && envelope.Error.Reason != "publish_daily") ||
		envelope.Error.RetryAfterSeconds != retrySeconds {
		return Result{Kind: ResultPermanent}
	}
	return Result{Kind: ResultRateLimited, RetryAfter: time.Duration(retrySeconds) * time.Second}
}

func validResponseHeaders(header http.Header, mediaType string) bool {
	return header.Get("Cache-Control") == "no-store" &&
		header.Get("X-Content-Type-Options") == "nosniff" &&
		header.Get("Content-Type") == mediaType
}

func readBoundedResponse(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maximumResponseBytes+1))
	if err != nil {
		clear(body)
		return nil, errors.New("read publisher response")
	}
	if len(body) > maximumResponseBytes {
		clear(body)
		return nil, errResponseTooLarge
	}
	return body, nil
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneEndpoint(value *metaserverv1.DirectEndpoint) *metaserverv1.DirectEndpoint {
	if value == nil {
		return nil
	}
	return &metaserverv1.DirectEndpoint{Hostname: value.Hostname, Port: value.Port}
}

func (kind ResultKind) String() string { return string(kind) }

func (result Result) validate() error {
	switch result.Kind {
	case ResultAccepted:
		if result.RetryAfter == 0 && result.MinimumNextSequence == 0 {
			return nil
		}
	case ResultReplay:
		if result.RetryAfter == 0 && result.MinimumNextSequence > 0 && result.RendezvousToken == [32]byte{} {
			return nil
		}
	case ResultRateLimited:
		if result.RetryAfter > 0 && result.RetryAfter <= maximumRetryAfter &&
			result.MinimumNextSequence == 0 && result.RendezvousToken == [32]byte{} {
			return nil
		}
	case ResultTransient:
		if result.RetryAfter >= 0 && result.RetryAfter <= maximumRetryAfter &&
			result.MinimumNextSequence == 0 && result.RendezvousToken == [32]byte{} {
			return nil
		}
	case ResultPermanent:
		if result.RetryAfter == 0 && result.MinimumNextSequence == 0 && result.RendezvousToken == [32]byte{} {
			return nil
		}
	default:
		return errors.New("unknown publisher result")
	}
	return errors.New("publisher result is internally inconsistent")
}
