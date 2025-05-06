package sources

type ModrinthPack struct {
	FormatVersion uint32             `json:"formatVersion"`
	Game          string             `json:"game"`
	VersionID     string             `json:"versionId"`
	Name          string             `json:"name"`
	Summary       string             `json:"summary,omitempty"`
	Files         []ModrinthPackFile `json:"files"`
	Dependencies  map[string]string  `json:"dependencies"`
}

type ModrinthPackFile struct {
	Path   string            `json:"path"`
	Hashes map[string]string `json:"hashes"`
	Env    *struct {
		Client string `json:"client"`
		Server string `json:"server"`
	} `json:"env"`
	Downloads []string `json:"downloads"`
	FileSize  uint32   `json:"fileSize"`
}
