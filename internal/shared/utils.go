package shared

import (
	"fmt"
	"os"
	"strings"
)

func GetRawForgeVersion(version string) string {
	var wantedVersion string
	// Check if we have a "-" in the version
	if strings.Contains(version, "-") {
		// We have a mcVersion-loaderVersion format
		// Strip the mcVersion
		wantedVersion = strings.Split(version, "-")[1]
	} else {
		wantedVersion = version
	}
	return wantedVersion
}

func Exitf(format string, a ...interface{}) {
	fmt.Printf(format, a...)
	os.Exit(1)
}

func Exitln(a ...interface{}) {
	fmt.Println(a...)
	os.Exit(1)
}
