package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultNuonAPIURL = "https://api.nuon.co"
	envNuonAPIURL     = "NUON_API_URL"
	envNuonAPIToken   = "NUON_API_TOKEN"
	envNuonOrgID      = "NUON_ORG_ID"
)

type NuonEnv struct {
	APIURL   string
	APIToken string
	OrgID    string
}

func LoadNuonEnv(getenv func(string) string) (NuonEnv, error) {
	if getenv == nil {
		getenv = os.Getenv
	}

	apiURL := strings.TrimSpace(getenv(envNuonAPIURL))
	if apiURL == "" {
		apiURL = defaultNuonAPIURL
	}

	apiToken := strings.TrimSpace(getenv(envNuonAPIToken))
	if apiToken == "" {
		return NuonEnv{}, fmt.Errorf("%s is required", envNuonAPIToken)
	}

	orgID := strings.TrimSpace(getenv(envNuonOrgID))
	if orgID == "" {
		return NuonEnv{}, fmt.Errorf("%s is required", envNuonOrgID)
	}

	return NuonEnv{
		APIURL:   apiURL,
		APIToken: apiToken,
		OrgID:    orgID,
	}, nil
}
