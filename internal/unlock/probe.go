package unlock

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/nexus/cfwarp-cli/internal/health"
)

const (
	ServiceGemini = "gemini"
	ServiceChatGPT = "chatgpt"
	ServiceClaude = "claude"
)

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

var geminiRegionPattern = regexp.MustCompile(`,2,1,200,"([A-Z]{3})"`)

type Status string

const (
	StatusAvailable    Status = "available"
	StatusUnavailable  Status = "unavailable"
	StatusWebOnly      Status = "web_only"
	StatusAppOnly      Status = "app_only"
	StatusUnknown      Status = "unknown"
	StatusNetworkError Status = "network_error"
)

type Config struct {
	ProxyMode string
	ProxyAddr string
	Username  string
	Password  string
	Timeout   time.Duration
}

type Result struct {
	Service string `json:"service"`
	Status  Status `json:"status"`
	OK      bool   `json:"ok"`
	Region  string `json:"region,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

func NormalizeServices(raw []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		for _, part := range strings.Split(item, ",") {
			name := normalizeService(part)
			if name == "" {
				continue
			}
			if !isSupported(name) {
				return nil, fmt.Errorf("unsupported unlock service %q", part)
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	return out, nil
}

func ProbeMany(ctx context.Context, cfg Config, services []string) ([]Result, error) {
	normalized, err := NormalizeServices(services)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(normalized))
	for _, service := range normalized {
		results = append(results, Probe(ctx, cfg, service))
	}
	return results, nil
}

func Probe(ctx context.Context, cfg Config, service string) Result {
	client, err := health.NewHTTPClient(cfg.ProxyMode, cfg.ProxyAddr, cfg.Username, cfg.Password, cfg.Timeout)
	if err != nil {
		return Result{Service: service, Status: StatusNetworkError, Detail: err.Error()}
	}
	return probeWithClient(ctx, client, normalizeService(service))
}

func probeWithClient(ctx context.Context, client *http.Client, service string) Result {
	switch service {
	case ServiceGemini:
		return probeGemini(ctx, client)
	case ServiceChatGPT:
		return probeChatGPT(ctx, client)
	case ServiceClaude:
		return probeClaude(ctx, client)
	default:
		return Result{Service: service, Status: StatusUnknown, Detail: "unsupported service"}
	}
}

func probeGemini(ctx context.Context, client *http.Client) Result {
	body, _, err := fetchBody(ctx, client, http.MethodGet, "https://gemini.google.com", func(r *http.Request) {
		r.Header.Set("User-Agent", browserUA)
	})
	if err != nil {
		return Result{Service: ServiceGemini, Status: StatusNetworkError, Detail: err.Error()}
	}
	status, region, detail := evaluateGemini(body)
	return Result{Service: ServiceGemini, Status: status, OK: status == StatusAvailable, Region: region, Detail: detail}
}

func probeChatGPT(ctx context.Context, client *http.Client) Result {
	cookieBody, _, err := fetchBody(ctx, client, http.MethodGet, "https://api.openai.com/compliance/cookie_requirements", func(r *http.Request) {
		r.Header.Set("Accept", "*/*")
		r.Header.Set("Authorization", "Bearer null")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "https://platform.openai.com")
		r.Header.Set("Referer", "https://platform.openai.com/")
		r.Header.Set("User-Agent", browserUA)
	})
	if err != nil {
		return Result{Service: ServiceChatGPT, Status: StatusNetworkError, Detail: err.Error()}
	}
	iosBody, _, err := fetchBody(ctx, client, http.MethodGet, "https://ios.chat.openai.com/", func(r *http.Request) {
		r.Header.Set("Accept", "*/*")
		r.Header.Set("User-Agent", browserUA)
	})
	if err != nil {
		return Result{Service: ServiceChatGPT, Status: StatusNetworkError, Detail: err.Error()}
	}
	status, detail := evaluateChatGPT(cookieBody, iosBody)
	return Result{Service: ServiceChatGPT, Status: status, OK: status == StatusAvailable, Detail: detail}
}

func probeClaude(ctx context.Context, client *http.Client) Result {
	_, finalURL, err := fetchBody(ctx, client, http.MethodGet, "https://claude.ai/", func(r *http.Request) {
		r.Header.Set("User-Agent", browserUA)
	})
	if err != nil {
		return Result{Service: ServiceClaude, Status: StatusNetworkError, Detail: err.Error()}
	}
	status, detail := evaluateClaude(finalURL)
	return Result{Service: ServiceClaude, Status: status, OK: status == StatusAvailable, Detail: detail}
}

func fetchBody(ctx context.Context, client *http.Client, method, target string, mutate func(*http.Request)) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return "", "", err
	}
	if mutate != nil {
		mutate(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", resp.Request.URL.String(), err
	}
	return string(body), resp.Request.URL.String(), nil
}

func evaluateGemini(body string) (Status, string, string) {
	if strings.Contains(body, "45631641,null,true") {
		region := ""
		if m := geminiRegionPattern.FindStringSubmatch(body); len(m) == 2 {
			region = m[1]
		}
		if region != "" {
			return StatusAvailable, region, "gemini available"
		}
		return StatusAvailable, "", "gemini available"
	}
	if strings.Contains(strings.ToLower(body), "not currently supported in your country") {
		return StatusUnavailable, "", "gemini region blocked"
	}
	return StatusUnavailable, "", "gemini marker missing"
}

func evaluateChatGPT(cookieBody, iosBody string) (Status, string) {
	unsupportedCountry := strings.Contains(strings.ToLower(cookieBody), "unsupported_country")
	vpnBlocked := strings.Contains(strings.ToLower(iosBody), "vpn")
	switch {
	case !unsupportedCountry && !vpnBlocked:
		return StatusAvailable, "chatgpt available"
	case unsupportedCountry && vpnBlocked:
		return StatusUnavailable, "chatgpt unavailable"
	case !unsupportedCountry && vpnBlocked:
		return StatusWebOnly, "chatgpt web-only"
	case unsupportedCountry && !vpnBlocked:
		return StatusAppOnly, "chatgpt app-only"
	default:
		return StatusUnknown, "chatgpt unknown"
	}
}

func evaluateClaude(finalURL string) (Status, string) {
	switch finalURL {
	case "https://claude.ai/", "https://claude.ai":
		return StatusAvailable, "claude available"
	case "https://www.anthropic.com/app-unavailable-in-region":
		return StatusUnavailable, "claude region blocked"
	default:
		if finalURL == "" {
			return StatusNetworkError, "claude request failed"
		}
		return StatusUnknown, finalURL
	}
}

func normalizeService(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "none":
		return ""
	case "chatgpt", "openai":
		return ServiceChatGPT
	case "gemini", "google-gemini":
		return ServiceGemini
	case "claude", "anthropic":
		return ServiceClaude
	default:
		return strings.ToLower(strings.TrimSpace(name))
	}
}

func isSupported(name string) bool {
	switch name {
	case ServiceGemini, ServiceChatGPT, ServiceClaude:
		return true
	default:
		return false
	}
}
