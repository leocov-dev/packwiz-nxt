package sources

import "github.com/leocov-dev/packwiz-nxt/core"

// RegisterAll registers every provider (CurseForge, GitHub, Modrinth) on reg. Library
// consumers building an isolated *core.Registry, instead of relying on
// core.DefaultRegistry (which each provider's init() populates automatically), should
// call this once on their own registry:
//
//	reg := core.NewRegistry()
//	sources.RegisterAll(reg)
func RegisterAll(reg *core.Registry) {
	RegisterCurseforge(reg)
	RegisterGithub(reg)
	RegisterModrinth(reg)
}
