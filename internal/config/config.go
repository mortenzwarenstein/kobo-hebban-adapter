package config

import (
	"encoding/json"
	"fmt"
	"os"

	"kobo-hebban-adapter/internal/kobo"
)

type Config struct {
	Port  string                    `json:"port"`
	Users map[string]kobo.UserEntry `json:"users"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	return &cfg, nil
}
