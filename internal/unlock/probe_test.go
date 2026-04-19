package unlock

import "testing"

func TestEvaluateGemini(t *testing.T) {
	status, region, _ := evaluateGemini(`foo 45631641,null,true bar ,2,1,200,"USA" baz`)
	if status != StatusAvailable || region != "USA" {
		t.Fatalf("expected gemini available/USA, got %q %q", status, region)
	}

	status, _, _ = evaluateGemini(`Gemini isn't currently supported in your country`)
	if status != StatusUnavailable {
		t.Fatalf("expected gemini unavailable, got %q", status)
	}
}

func TestEvaluateChatGPT(t *testing.T) {
	cases := []struct {
		name   string
		cookie string
		ios    string
		want   Status
	}{
		{name: "available", cookie: `{}`, ios: `ok`, want: StatusAvailable},
		{name: "unavailable", cookie: `unsupported_country`, ios: `VPN`, want: StatusUnavailable},
		{name: "web only", cookie: `{}`, ios: `VPN`, want: StatusWebOnly},
		{name: "app only", cookie: `unsupported_country`, ios: `ok`, want: StatusAppOnly},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := evaluateChatGPT(tc.cookie, tc.ios)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestEvaluateClaude(t *testing.T) {
	cases := []struct {
		url  string
		want Status
	}{
		{url: "https://claude.ai/", want: StatusAvailable},
		{url: "https://www.anthropic.com/app-unavailable-in-region", want: StatusUnavailable},
		{url: "https://claude.ai/login", want: StatusUnknown},
	}
	for _, tc := range cases {
		got, _ := evaluateClaude(tc.url)
		if got != tc.want {
			t.Fatalf("url %q: expected %q, got %q", tc.url, tc.want, got)
		}
	}
}

func TestEvaluateNetflix(t *testing.T) {
	status, region, _ := evaluateNetflix(`{ "countryName":"US", "id":"USA" }`, `{ "countryName":"US" }`)
	if status != StatusAvailable || region == "" {
		t.Fatalf("expected netflix available with region, got %q %q", status, region)
	}

	status, _, detail := evaluateNetflix(`foo Oh no! bar`, `another Oh no! entry`)
	if status != StatusWebOnly {
		t.Fatalf("expected netflix originals only, got %q", status)
	}
	if detail == "" {
		t.Fatalf("expected detail for netflix originals only")
	}
}

func TestEvaluateYouTube(t *testing.T) {
	status, region, _ := evaluateYouTubePremium(`{"INNERTUBE_CONTEXT_GL":"US"} ad-free`)
	if status != StatusAvailable || region != "US" {
		t.Fatalf("expected youtube available in US, got %q %q", status, region)
	}

	status, _, _ = evaluateYouTubePremium(`anywhere premium is not available in your country`)
	if status != StatusUnavailable {
		t.Fatalf("expected youtube unavailable, got %q", status)
	}

	status, region, _ = evaluateYouTubePremium(`visit www.google.cn for more`)
	if status != StatusUnavailable || region != "CN" {
		t.Fatalf("expected youtube CN unavailable, got %q %q", status, region)
	}
}

func TestNormalizeServices(t *testing.T) {
	got, err := NormalizeServices([]string{"gemini,openai", "claude", "chatgpt", "netflix", "youtube"})
	if err != nil {
		t.Fatalf("normalize services: %v", err)
	}
	if len(got) != 5 || got[0] != ServiceGemini || got[1] != ServiceChatGPT || got[2] != ServiceClaude || got[3] != ServiceNetflix || got[4] != ServiceYouTube {
		t.Fatalf("unexpected normalized services: %#v", got)
	}
}
