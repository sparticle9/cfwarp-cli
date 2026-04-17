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

func TestNormalizeServices(t *testing.T) {
	got, err := NormalizeServices([]string{"gemini,openai", "claude", "chatgpt"})
	if err != nil {
		t.Fatalf("normalize services: %v", err)
	}
	if len(got) != 3 || got[0] != ServiceGemini || got[1] != ServiceChatGPT || got[2] != ServiceClaude {
		t.Fatalf("unexpected normalized services: %#v", got)
	}
}
