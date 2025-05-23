package config

import (
	"encoding/base64"
	"errors"
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
	k, err := base64.StdEncoding.DecodeString(cfApiKey)
	if err != nil || len(k) == 0 {
		return "", errors.New("failed to decode CF API key")
	}
	return string(k), nil
}

func GetGhApiKey() string {
	return ghApiKey
}
