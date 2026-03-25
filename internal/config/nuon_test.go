package config

import "testing"

func TestLoadNuonEnvDefaultsAPIURL(t *testing.T) {
	env, err := LoadNuonEnv(func(key string) string {
		switch key {
		case envNuonAPIToken:
			return "tok_123"
		case envNuonOrgID:
			return "org_123"
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if env.APIURL != defaultNuonAPIURL {
		t.Fatalf("expected default api url %q, got %q", defaultNuonAPIURL, env.APIURL)
	}
}
