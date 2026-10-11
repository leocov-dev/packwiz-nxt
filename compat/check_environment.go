package compat

import (
	"fmt"

	"github.com/leocov-dev/packwiz-nxt/core"
	"github.com/leocov-dev/packwiz-nxt/jarmeta"
)

// checkEnvironment reports a jar whose declared environment (Fabric/Quilt)
// does not fit the pack side of the entry.
func checkEnvironment(_ *checkContext, e *resolved) []Finding {
	env := e.meta.Environment
	switch {
	case env == jarmeta.EnvClient && e.Side == core.ServerSide,
		env == jarmeta.EnvServer && e.Side == core.ClientSide:
		return []Finding{{
			Severity: SeverityInfo, Code: CodeSideMismatch, Slug: e.Slug,
			Message: fmt.Sprintf("mod declares environment %q but is set to side %q in the pack", env, e.Side),
			Detail:  map[string]string{"environment": string(env), "side": string(e.Side)},
		}}
	}
	return nil
}
