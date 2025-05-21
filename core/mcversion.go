package core

import (
	"encoding/json"
	"golang.org/x/exp/slices"
	"io"
)

type versionJson struct {
	Latest struct {
		Release  string `json:"release"`
		Snapshot string `json:"snapshot"`
	} `json:"latest"`
	Versions []versionDef `json:"versions"`
}

type versionDef struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	Time        string `json:"time"`
	ReleaseTime string `json:"releaseTime"`
}

type McVersionInfo struct {
	Latest         string
	LatestSnapshot string
	Versions       []string
}

func (m McVersionInfo) CheckValid(version string) bool {
	return slices.Contains(m.Versions, version)
}

func GetMinecraftVersions() (McVersionInfo, error) {
	var versionInfo McVersionInfo

	resp, err := GetWithUA("https://launchermeta.mojang.com/mc/game/version_manifest.json", "application/json")
	if err != nil {
		return versionInfo, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return versionInfo, err
	}

	var info versionJson
	if err := json.Unmarshal(body, &info); err != nil {
		return versionInfo, err
	}

	versions := make([]string, 0)

	for _, v := range info.Versions {
		if v.Type != "release" {
			continue
		}
		versions = append(versions, v.ID)
	}

	versionInfo = McVersionInfo{
		Latest:         info.Latest.Release,
		LatestSnapshot: info.Latest.Snapshot,
		Versions:       versions,
	}

	return versionInfo, nil
}
