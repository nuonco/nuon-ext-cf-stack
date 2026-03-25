package config

import (
	"encoding/json"
	"fmt"
	"os"
)

func LoadJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("parse %s as JSON object: %w", path, err)
	}

	if value == nil {
		value = map[string]any{}
	}

	return value, nil
}
