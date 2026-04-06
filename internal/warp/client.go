package warp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL   = "https://api.cloudflareclient.com"
	registrationPath = "/v0a2158/reg"
	userAgent        = "okhttp/3.12.1"
	cfClientVersion  = "a-6.3-2158"
	defaultTimeout   = 30 * time.Second
)

// Client calls the Cloudflare WARP consumer API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a Client using the production Cloudflare endpoint.
func NewClient() *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// NewClientWithBase is used in tests to point at a mock server.
func NewClientWithBase(baseURL string, hc *http.Client) *Client {
	return &Client{baseURL: baseURL, httpClient: hc}
}

// RegistrationRequest is the body sent to the WARP registration API.
type registrationRequest struct {
	Key  string `json:"key"`
	TOS  string `json:"tos"`
	Type string `json:"type"`
}

// registrationResponse is the subset of the WARP API response we care about.
type registrationResponse struct {
	ID    string `json:"id"`
	Token string `json:"token"`
	Account struct {
		License string `json:"license"`
	} `json:"account"`
	Config struct {
		ClientID string `json:"client_id"`
		Reserved [3]int `json:"reserved"`
		Peers    []struct {
			PublicKey string `json:"public_key"`
			Endpoint  struct {
				V4   string `json:"v4"`
				V6   string `json:"v6"`
				Host string `json:"host"`
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

// RegistrationResult is the parsed outcome of a successful registration call.
type RegistrationResult struct {
	AccountID    string
	Token        string
	License      string
	ClientID     string
	Reserved     [3]int
	PeerPublicKey string
	PeerEndpoint  string // "host:port" from config.peers[0].endpoint.v4, fallback to host
	IPv4          string
	IPv6          string
}

// Register calls the Cloudflare consumer registration API with the given public key.
func (c *Client) Register(ctx context.Context, publicKey string) (RegistrationResult, error) {
	body := registrationRequest{
		Key:  publicKey,
		TOS:  time.Now().UTC().Format(time.RFC3339),
		Type: "a",
	}

	data, err := json.Marshal(body)
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+registrationPath, bytes.NewReader(data))
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("CF-Client-Version", cfClientVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("registration request failed: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return RegistrationResult{}, fmt.Errorf("registration API returned %d: %s", resp.StatusCode, truncate(string(rawBody), 256))
	}

	var parsed registrationResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return RegistrationResult{}, fmt.Errorf("parse registration response: %w", err)
	}
	if err := validateResponse(parsed); err != nil {
		return RegistrationResult{}, err
	}

	peer := parsed.Config.Peers[0]
	peerEndpoint := peer.Endpoint.V4
	if peerEndpoint == "" {
		peerEndpoint = peer.Endpoint.Host
	}

	return RegistrationResult{
		AccountID:     parsed.ID,
		Token:         parsed.Token,
		License:       parsed.Account.License,
		ClientID:      parsed.Config.ClientID,
		Reserved:      parsed.Config.Reserved,
		PeerPublicKey: peer.PublicKey,
		PeerEndpoint:  peerEndpoint,
		IPv4:          parsed.Config.Interface.Addresses.V4,
		IPv6:          parsed.Config.Interface.Addresses.V6,
	}, nil
}

func validateResponse(r registrationResponse) error {
	switch {
	case r.ID == "":
		return fmt.Errorf("registration response missing account id")
	case r.Token == "":
		return fmt.Errorf("registration response missing token")
	case len(r.Config.Peers) == 0 || r.Config.Peers[0].PublicKey == "":
		return fmt.Errorf("registration response missing peer public key")
	case r.Config.Interface.Addresses.V4 == "":
		return fmt.Errorf("registration response missing IPv4 address")
	case r.Config.Interface.Addresses.V6 == "":
		return fmt.Errorf("registration response missing IPv6 address")
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
