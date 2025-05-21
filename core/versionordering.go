package core

import (
	"sort"
	"strconv"
	"strings"
)

// CompareVersions compares two version strings and returns:
//
//	-1 if version1 < version2
//	 0 if version1 == version2
//	 1 if version1 > version2
func CompareVersions(version1, version2 string) int {
	// Split versions into parts
	v1Parts := strings.Split(version1, ".")
	v2Parts := strings.Split(version2, ".")

	// Find the maximum length between the two version arrays
	maxLen := len(v1Parts)
	if len(v2Parts) > maxLen {
		maxLen = len(v2Parts)
	}

	// Pad shorter version with "0"s
	for len(v1Parts) < maxLen {
		v1Parts = append(v1Parts, "0")
	}
	for len(v2Parts) < maxLen {
		v2Parts = append(v2Parts, "0")
	}

	// Compare each part
	for i := 0; i < maxLen; i++ {
		// Convert string parts to integers for proper numeric comparison
		num1, err1 := strconv.Atoi(v1Parts[i])
		num2, err2 := strconv.Atoi(v2Parts[i])

		// Handle conversion errors (if any part is not a valid integer)
		if err1 != nil || err2 != nil {
			// Fall back to string comparison if conversion fails
			if v1Parts[i] < v2Parts[i] {
				return -1
			}
			if v1Parts[i] > v2Parts[i] {
				return 1
			}
			continue
		}

		if num1 < num2 {
			return -1
		}
		if num1 > num2 {
			return 1
		}
	}

	return 0 // Versions are equal
}

func SortAscending(versions []string) []string {
	sort.Slice(versions, func(i, j int) bool {
		return CompareVersions(versions[i], versions[j]) == -1
	})
	return versions
}

func SortDescending(versions []string) []string {
	sort.Slice(versions, func(i, j int) bool {
		return CompareVersions(versions[i], versions[j]) == 1
	})
	return versions
}
