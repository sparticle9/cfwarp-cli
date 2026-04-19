package unlock

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/nexus/cfwarp-cli/internal/health"
)

const (
	ServiceGemini  = "gemini"
	ServiceChatGPT = "chatgpt"
	ServiceClaude  = "claude"
	ServiceNetflix = "netflix"
	ServiceYouTube = "youtube"
)

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

var (
	geminiRegionPattern         = regexp.MustCompile(`,2,1,200,"([A-Z]{3})"`)
	netflixCountryNamePattern   = regexp.MustCompile(`"countryName"\s*:\s*"([^"]+)"`)
	netflixCountryCodePattern   = regexp.MustCompile(`"id"\s*:\s*"([^"]+)"`)
	netflixRegionCodePattern    = regexp.MustCompile(`"countryCode"\s*:\s*"([^"]+)"`)
	youtubeContextRegionPattern = regexp.MustCompile(`"INNERTUBE_CONTEXT_GL"\s*:\s*"([A-Z]{2})"`)
	cdnDomainPattern            = regexp.MustCompile(`"url"\s*:\s*"([^\"]+)"`)
	netflixISPPattern           = regexp.MustCompile(`"isp"\s*:\s*"([^"]+)"`)
	netflixCountryPattern       = regexp.MustCompile(`"country"\s*:\s*"([^"]+)"`)
)

// Optional supplement metadata returned by service probes.
type Supplement map[string]string

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
	Service    string     `json:"service"`
	Status     Status     `json:"status"`
	OK         bool       `json:"ok"`
	Region     string     `json:"region,omitempty"`
	Detail     string     `json:"detail,omitempty"`
	Supplement Supplement `json:"supplement,omitempty"`
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
	case ServiceNetflix:
		return probeNetflix(ctx, client)
	case ServiceYouTube:
		return probeYouTube(ctx, client)
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

func probeNetflix(ctx context.Context, client *http.Client) Result {
	mutate := func(r *http.Request) {
		r.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
		r.Header.Set("Accept-Language", "en-US,en;q=0.9")
		r.Header.Set("User-Agent", browserUA)
	}
	titleA, _, err := fetchBody(ctx, client, http.MethodGet, "https://www.netflix.com/title/81280792", mutate)
	if err != nil {
		return Result{Service: ServiceNetflix, Status: StatusNetworkError, Detail: err.Error()}
	}
	titleB, _, err := fetchBody(ctx, client, http.MethodGet, "https://www.netflix.com/title/70143836", mutate)
	if err != nil {
		return Result{Service: ServiceNetflix, Status: StatusNetworkError, Detail: err.Error()}
	}
	status, region, detail := evaluateNetflix(titleA, titleB)
	result := Result{Service: ServiceNetflix, Status: status, OK: status == StatusAvailable, Region: region, Detail: detail}

	supplement := make(map[string]string)
	cdnInfo := probeNetflixCDN(ctx, client)
	if cdnInfo != "" {
		supplement["netflix-cdn"] = cdnInfo
		result.Supplement = supplement
	}
	return result
}

func probeYouTube(ctx context.Context, client *http.Client) Result {
	body, _, err := fetchBody(ctx, client, http.MethodGet, "https://www.youtube.com/premium", func(r *http.Request) {
		r.Header.Set("Accept-Language", "en-US,en;q=0.9")
		r.Header.Set("User-Agent", browserUA)
		r.Header.Set("Cookie", "VISITOR_INFO1_LIVE=Di84mAIbgKY; YSC=FSCWhKo2Zgw; __Secure-YEC=CgtRWTBGTFExeV9Iayjele2yBjIKCgJERRIEEgAgYQ%3D%3D")
	})
	if err != nil {
		return Result{Service: ServiceYouTube, Status: StatusNetworkError, Detail: err.Error()}
	}
	status, region, detail := evaluateYouTubePremium(body)
	return Result{Service: ServiceYouTube, Status: status, OK: status == StatusAvailable, Region: region, Detail: detail}
}

func probeNetflixCDN(ctx context.Context, client *http.Client) string {
	body, _, statusCode, err := fetchBodyWithStatus(ctx, client, http.MethodGet, "https://api.fast.com/netflix/speedtest/v2?https=true&token=YXNkZmFzZGxmbnNkYWZoYXNkZmhrYWxm&urlCount=1", func(r *http.Request) {
		r.Header.Set("User-Agent", browserUA)
	})
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Sprintf("%v", ctx.Err())
		}
		return "network error"
	}
	if statusCode == http.StatusForbidden {
		return "blocked by netflix"
	}
	if statusCode < 200 || statusCode >= 400 {
		return fmt.Sprintf("http_%d", statusCode)
	}
	cdnURL := extractFirst(cdnDomainPattern, body)
	if cdnURL == "" {
		return "page error"
	}
	u, err := url.Parse(cdnURL)
	if err != nil || u.Host == "" {
		return "page error"
	}
	ips, err := net.LookupIP(u.Hostname())
	if err != nil || len(ips) == 0 {
		return "cdn ip not found"
	}
	cdnIP := ""
	for _, ip := range ips {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() {
			continue
		}
		cdnIP = ip.String()
		break
	}
	if cdnIP == "" {
		return "cdn ip hidden"
	}
	geoBody, _, err := fetchBody(ctx, client, http.MethodGet, "https://api.ip.sb/geoip/"+cdnIP, func(r *http.Request) {
		r.Header.Set("User-Agent", browserUA)
	})
	if err != nil {
		return fmt.Sprintf("ip=%s lookup failed", cdnIP)
	}

	country := extractFirst(netflixCountryPattern, geoBody)
	isp := extractFirst(netflixISPPattern, geoBody)
	if isp == "" {
		return "no isp info"
	}
	if country == "" {
		country = cdnIP
	}
	if isp == "Netflix Streaming Services" {
		return country
	}
	return fmt.Sprintf("%s (%s)", country, isp)
}

func fetchBody(ctx context.Context, client *http.Client, method, target string, mutate func(*http.Request)) (string, string, error) {
	body, finalURL, _, err := fetchBodyWithStatus(ctx, client, method, target, mutate)
	return body, finalURL, err
}

func fetchBodyWithStatus(ctx context.Context, client *http.Client, method, target string, mutate func(*http.Request)) (string, string, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return "", "", 0, err
	}
	if mutate != nil {
		mutate(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return "", resp.Request.URL.String(), resp.StatusCode, err
	}
	return string(body), resp.Request.URL.String(), resp.StatusCode, nil
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

func evaluateNetflix(titleA, titleB string) (Status, string, string) {
	titleAUnavailable := strings.Contains(titleA, "Oh no!")
	titleBUnavailable := strings.Contains(titleB, "Oh no!")

	if titleAUnavailable && titleBUnavailable {
		return StatusWebOnly, "", "netflix originals only"
	}
	if !titleAUnavailable || !titleBUnavailable {
		region := extractNetflixRegion(titleA)
		if region == "" {
			region = extractNetflixRegion(titleB)
		}
		if region == "" {
			region = "UNKNOWN"
		}
		return StatusAvailable, region, "netflix available"
	}
	return StatusUnavailable, "", "netflix unavailable"
}

func evaluateYouTubePremium(body string) (Status, string, string) {
	if strings.Contains(body, "www.google.cn") {
		return StatusUnavailable, "CN", "youtube premium unavailable in CN"
	}
	if strings.Contains(strings.ToLower(body), "premium is not available in your country") {
		return StatusUnavailable, "", "youtube premium unavailable"
	}
	if strings.Contains(body, "ad-free") {
		region := extractYouTubePremiumRegion(body)
		if region == "" {
			region = "UNKNOWN"
		}
		return StatusAvailable, region, "youtube premium available"
	}
	return StatusUnknown, "", "youtube page parsing failed"
}

func extractNetflixRegion(body string) string {
	if m := netflixCountryNamePattern.FindStringSubmatch(body); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	if m := netflixRegionCodePattern.FindStringSubmatch(body); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	if m := netflixCountryCodePattern.FindStringSubmatch(body); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractYouTubePremiumRegion(body string) string {
	if m := youtubeContextRegionPattern.FindStringSubmatch(body); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractFirst(re *regexp.Regexp, text string) string {
	if re == nil {
		return ""
	}
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
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
	case "youtube", "youtube-premium", "yt":
		return ServiceYouTube
	case "netflix":
		return ServiceNetflix
	default:
		return strings.ToLower(strings.TrimSpace(name))
	}
}

func isSupported(name string) bool {
	switch name {
	case ServiceGemini, ServiceChatGPT, ServiceClaude, ServiceNetflix, ServiceYouTube:
		return true
	default:
		return false
	}
}
