package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/warp"
)

const (
	defaultBaseURL  = "https://api.cloudflareclient.com"
	registrationAPI = "/v0a2158/reg"
	userAgent       = "okhttp/3.12.1"
	clientVersion   = "a-6.3-2158"
	defaultTimeout  = 30 * time.Second
)

const (
	masqueKeyType    = "secp256r1"
	masqueTunnelType = "masque"
)

// Client wraps the Cloudflare consumer registration APIs needed by both
// WireGuard and MASQUE control-plane flows.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{baseURL: defaultBaseURL, httpClient: &http.Client{Timeout: defaultTimeout}}
}

func NewClientWithBase(baseURL string, hc *http.Client) *Client {
	return &Client{baseURL: baseURL, httpClient: hc}
}

// RegisterConsumer proxies to the existing consumer registration logic.
func (c *Client) RegisterConsumer(ctx context.Context, publicKey string) (warp.RegistrationResult, error) {
	return warp.NewClientWithBase(c.baseURL, c.httpClient).Register(ctx, publicKey)
}

// MasqueEnrollmentResult is the MASQUE-specific state returned by the PATCH flow.
type MasqueEnrollmentResult struct {
	EndpointPubKeyPEM string
	EndpointV4        string
	EndpointV6        string
	IPv4              string
	IPv6              string
}

type masqueEnrollRequest struct {
	Key        string `json:"key"`
	KeyType    string `json:"key_type"`
	TunnelType string `json:"tunnel_type"`
	Name       string `json:"name,omitempty"`
}

type masqueEnrollResponse struct {
	Config struct {
		Peers []struct {
			PublicKey string `json:"public_key"`
			Endpoint  struct {
				V4 string `json:"v4"`
				V6 string `json:"v6"`
			} `json:"endpoint"`
		} `json:"peers"`
		Interface struct {
			Addresses struct {
				V4 string `json:"v4"`
				V6 string `json:"v6"`
			} `json:"addresses"`
		} `json:"interface"`
	} `json:"config"`
}

// EnrollMasqueKey switches an existing consumer registration to MASQUE mode and
// returns the resulting MASQUE-specific endpoint and address material.
func (c *Client) EnrollMasqueKey(ctx context.Context, accountID, token, publicKeyBase64 string, deviceName string) (MasqueEnrollmentResult, error) {
	body := masqueEnrollRequest{
		Key:        publicKeyBase64,
		KeyType:    masqueKeyType,
		TunnelType: masqueTunnelType,
		Name:       deviceName,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return MasqueEnrollmentResult{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.baseURL+registrationAPI+"/"+accountID, bytes.NewReader(data))
	if err != nil {
		return MasqueEnrollmentResult{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("CF-Client-Version", clientVersion)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return MasqueEnrollmentResult{}, fmt.Errorf("enrollment request failed: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return MasqueEnrollmentResult{}, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return MasqueEnrollmentResult{}, fmt.Errorf("enrollment API returned %d: %s", resp.StatusCode, truncate(string(rawBody), 256))
	}

	var parsed masqueEnrollResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return MasqueEnrollmentResult{}, fmt.Errorf("parse enrollment response: %w", err)
	}
	if err := validateMasqueEnrollment(parsed); err != nil {
		return MasqueEnrollmentResult{}, err
	}

	peer := parsed.Config.Peers[0]
	return MasqueEnrollmentResult{
		EndpointPubKeyPEM: peer.PublicKey,
		EndpointV4:        peer.Endpoint.V4,
		EndpointV6:        peer.Endpoint.V6,
		IPv4:              parsed.Config.Interface.Addresses.V4,
		IPv6:              parsed.Config.Interface.Addresses.V6,
	}, nil
}

func validateMasqueEnrollment(r masqueEnrollResponse) error {
	switch {
	case len(r.Config.Peers) == 0:
		return fmt.Errorf("enrollment response missing peers")
	case r.Config.Peers[0].PublicKey == "":
		return fmt.Errorf("enrollment response missing endpoint public key")
	case r.Config.Interface.Addresses.V4 == "":
		return fmt.Errorf("enrollment response missing IPv4 address")
	}
	return nil
}

// BuildMasqueState converts enrollment output and locally generated key material
// into the persisted transport-aware MASQUE account state.
func BuildMasqueState(privateKeyDERBase64 string, result MasqueEnrollmentResult) *state.MasqueState {
	return &state.MasqueState{
		PrivateKeyDERBase64: privateKeyDERBase64,
		EndpointPubKeyPEM:   result.EndpointPubKeyPEM,
		EndpointV4:          result.EndpointV4,
		EndpointV6:          result.EndpointV6,
		IPv4:                result.IPv4,
		IPv6:                result.IPv6,
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
