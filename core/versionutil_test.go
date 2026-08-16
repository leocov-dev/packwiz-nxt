package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNeoforgeMavenVersionToKey(t *testing.T) {
	cases := []struct {
		name        string
		version     string
		expectedKey string
	}{
		{"old scheme, patch-only", "20.6.2", "1.20.6"},
		{"old scheme, beta suffix", "21.10.43-beta", "1.21.10"},
		{"old scheme, distinct from 1.21.10", "21.1.213", "1.21.1"},
		{"new scheme, snapshot build 6", "26.1.0.0-alpha.9+snapshot-6", "26.1-snapshot-6"},
		{"new scheme, same snapshot bucket as build 6", "26.1.0.0-alpha.10+snapshot-6", "26.1-snapshot-6"},
		{"new scheme, distinct snapshot bucket", "26.1.0.0-alpha.11+snapshot-7", "26.1-snapshot-7"},
		{"new scheme, hypothetical stable release", "26.2.3.0", "26.2.3"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, loaderVersion := neoforgeMavenVersionToKey(tc.version)
			assert.Equal(t, tc.expectedKey, key)
			assert.Equal(t, tc.version, loaderVersion)
		})
	}
}

// TestNeoforgeMavenVersionToKey_NoCollision guards against the collision upstream packwiz
// hit (MC 1.21.1 pulling in NeoForge versions meant for 1.21.10): distinct old-scheme
// Minecraft versions must always map to distinct keys.
func TestNeoforgeMavenVersionToKey_NoCollision(t *testing.T) {
	keyA, _ := neoforgeMavenVersionToKey("21.1.212")
	keyB, _ := neoforgeMavenVersionToKey("21.10.43-beta")
	assert.NotEqual(t, keyA, keyB)
	assert.Equal(t, "1.21.1", keyA)
	assert.Equal(t, "1.21.10", keyB)
}

// TestNeoforgeMavenVersionToKey_Merge mirrors fetchMavenMap's merge loop, feeding a batch of
// raw maven version strings through neoforgeMavenVersionToKey and checking the resulting
// buckets, without touching HTTP.
func TestNeoforgeMavenVersionToKey_Merge(t *testing.T) {
	rawVersions := []string{
		"20.6.2",
		"20.6.83-beta",
		"21.10.43-beta",
		"26.1.0.0-alpha.9+snapshot-6",
		"26.1.0.0-alpha.10+snapshot-6",
		"26.1.0.0-alpha.11+snapshot-7",
	}

	buckets := make(map[string][]string)
	for _, v := range rawVersions {
		key, loaderVersion := neoforgeMavenVersionToKey(v)
		if key == "" {
			continue
		}
		buckets[key] = append(buckets[key], loaderVersion)
	}

	assert.ElementsMatch(t, []string{"20.6.2", "20.6.83-beta"}, buckets["1.20.6"])
	assert.ElementsMatch(t, []string{"21.10.43-beta"}, buckets["1.21.10"])
	assert.ElementsMatch(t, []string{
		"26.1.0.0-alpha.9+snapshot-6",
		"26.1.0.0-alpha.10+snapshot-6",
	}, buckets["26.1-snapshot-6"])
	assert.ElementsMatch(t, []string{"26.1.0.0-alpha.11+snapshot-7"}, buckets["26.1-snapshot-7"])
}
