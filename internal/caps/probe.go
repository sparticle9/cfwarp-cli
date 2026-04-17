package caps

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nexus/cfwarp-cli/internal/health"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/unlock"
)

const (
	ProbeInternet = "internet"
	ProbeWarp     = "warp"
	ProbeGemini   = "gemini"
	ProbeChatGPT  = "chatgpt"
)

type ProbeTarget struct {
	Type     string
	Address  string
	Username string
	Password string
}

type Result struct {
	Probe   string            `json:"probe"`
	OK      bool              `json:"ok"`
	Region  string            `json:"region,omitempty"`
	Detail  string            `json:"detail,omitempty"`
	Fields  map[string]string `json:"fields,omitempty"`
	Status  string            `json:"status,omitempty"`
	Checked time.Time         `json:"checked_at"`
}

func ProbeCheck(ctx context.Context, target ProbeTarget, check state.CapCheck) Result {
	probe := check.Probe
	timeout := time.Duration(check.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	checked := time.Now().UTC()
	switch probe {
	case ProbeInternet:
		return probeInternet(ctx, target, timeout, checked)
	case ProbeWarp:
		return probeWarp(ctx, target, timeout, checked)
	case ProbeGemini, ProbeChatGPT:
		return probeUnlock(ctx, target, probe, timeout, checked)
	default:
		return Result{Probe: probe, OK: false, Detail: "unsupported probe", Checked: checked}
	}
}

func probeInternet(ctx context.Context, target ProbeTarget, timeout time.Duration, checked time.Time) Result {
	client, err := health.NewHTTPClient(target.Type, target.Address, target.Username, target.Password, timeout)
	if err != nil {
		return Result{Probe: ProbeInternet, OK: false, Detail: err.Error(), Checked: checked}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.google.com/generate_204", nil)
	if err != nil {
		return Result{Probe: ProbeInternet, OK: false, Detail: err.Error(), Checked: checked}
	}
	resp, err := client.Do(req)
	if err != nil {
		return Result{Probe: ProbeInternet, OK: false, Detail: err.Error(), Checked: checked}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8*1024))
	ok := resp.StatusCode == http.StatusNoContent || (resp.StatusCode >= 200 && resp.StatusCode < 400)
	return Result{Probe: ProbeInternet, OK: ok, Detail: fmt.Sprintf("http_status=%d", resp.StatusCode), Checked: checked}
}

func probeWarp(ctx context.Context, target ProbeTarget, timeout time.Duration, checked time.Time) Result {
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	trace, err := health.ProbeTrace(probeCtx, target.Type, target.Address, target.Username, target.Password)
	if err != nil {
		return Result{Probe: ProbeWarp, OK: false, Detail: err.Error(), Checked: checked}
	}
	return Result{Probe: ProbeWarp, OK: trace.WARPOn, Detail: fmt.Sprintf("warp=%s", trace.Fields["warp"]), Region: trace.Fields["colo"], Fields: trace.Fields, Checked: checked}
}

func probeUnlock(ctx context.Context, target ProbeTarget, probe string, timeout time.Duration, checked time.Time) Result {
	result := unlock.Probe(ctx, unlock.Config{
		ProxyMode: target.Type,
		ProxyAddr: target.Address,
		Username:  target.Username,
		Password:  target.Password,
		Timeout:   timeout,
	}, probe)
	return Result{Probe: probe, OK: result.OK, Region: result.Region, Detail: result.Detail, Status: string(result.Status), Checked: checked}
}
