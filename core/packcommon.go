package core

import (
	"errors"
	"fmt"
)

// This file holds logic shared between Pack (the in-memory domain type) and
// PackToml (the TOML wire-format type), which both store the same
// minecraft-version/options/loaders data and previously duplicated the
// methods below almost verbatim. Both types delegate to these free functions
// operating directly on the common field types instead.

// mcVersionFrom gets the Minecraft version out of a pack's Versions map, if set.
func mcVersionFrom(versions map[string]string) (string, error) {
	mcVersion, ok := versions["minecraft"]
	if !ok {
		return "", errors.New("no minecraft version specified in modpack")
	}
	return mcVersion, nil
}

// acceptableGameVersionsFrom returns a pack's "acceptable-game-versions" option as a
// []string. TOML-decoded options stored as interface{} may come back as either
// []string (if set programmatically) or []interface{} (if decoded from a TOML file),
// so both are handled here instead of panicking on an unchecked type assertion.
func acceptableGameVersionsFrom(options map[string]interface{}) ([]string, error) {
	acceptableVersionsRaw, ok := options["acceptable-game-versions"]
	if !ok {
		return []string{}, nil
	}
	if versions, ok := acceptableVersionsRaw.([]string); ok {
		return versions, nil
	}
	if versionsAny, ok := acceptableVersionsRaw.([]interface{}); ok {
		versions := make([]string, 0, len(versionsAny))
		for _, v := range versionsAny {
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("acceptable-game-versions contains a non-string value: %v", v)
			}
			versions = append(versions, s)
		}
		return versions, nil
	}
	return nil, fmt.Errorf("acceptable-game-versions has an unexpected type: %T", acceptableVersionsRaw)
}

// setAcceptableGameVersions sorts, dedupes, and stores versions into options.
func setAcceptableGameVersions(options map[string]interface{}, versions []string) {
	SortAndDedupeVersions(versions)
	options["acceptable-game-versions"] = versions
}

// supportedMCVersionsFrom gets the versions of Minecraft a pack allows in downloaded
// mods, ordered by preference (highest = most desirable).
func supportedMCVersionsFrom(versions map[string]string, options map[string]interface{}) ([]string, error) {
	mcVersion, err := mcVersionFrom(versions)
	if err != nil {
		return nil, err
	}
	acceptableVersions, err := acceptableGameVersionsFrom(options)
	if err != nil {
		return nil, err
	}
	allVersions := append(append([]string(nil), acceptableVersions...), mcVersion)
	SortAndDedupeVersions(allVersions)
	return allVersions, nil
}

// compatibleLoadersFrom returns the pack's loaders, including backwards-compatible
// aliases (quilt implies fabric, neoforge implies forge).
func compatibleLoadersFrom(versions map[string]string) (loaders []string) {
	if _, hasQuilt := versions["quilt"]; hasQuilt {
		loaders = append(loaders, "quilt")
		loaders = append(loaders, "fabric") // Backwards-compatible; for now (could be configurable later)
	} else if _, hasFabric := versions["fabric"]; hasFabric {
		loaders = append(loaders, "fabric")
	}
	if _, hasNeoForge := versions["neoforge"]; hasNeoForge {
		loaders = append(loaders, "neoforge")
		loaders = append(loaders, "forge") // Backwards-compatible; for now (could be configurable later)
	} else if _, hasForge := versions["forge"]; hasForge {
		loaders = append(loaders, "forge")
	}
	return
}
