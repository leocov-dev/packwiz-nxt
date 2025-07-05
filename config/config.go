package config

import (
	"encoding/base64"
	"errors"
	"fmt"
)

var (
	Version  string
	cfApiKey string
	ghApiKey string
)

func SetVersion(version string) {
	Version = version
}

func SetCurseforgeApiKey(key string) {
	cfApiKey = key
}

func SetGitHubApiKey(key string) {
	ghApiKey = key
}

func DecodeCfApiKey() (string, error) {
	if cfApiKey == "" {
		return "", errors.New("CF API key not set")
	}
	k, err := base64.StdEncoding.DecodeString(cfApiKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode CF API key: %w", err)
	}
	if len(k) == 0 {
		return "", errors.New("CF API key decoded to empty string")
	}
	return string(k), nil
}

func GetGhApiKey() string {
	return ghApiKey
}
