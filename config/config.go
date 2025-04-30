package config

var (
	Version  string
	CfApiKey string
)

func SetConfig(
	version string,
	cfApiKey string,
) {
	Version = version
	CfApiKey = cfApiKey
}
