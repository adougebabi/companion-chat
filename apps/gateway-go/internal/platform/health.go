// Package platform contains transport-neutral contracts shared by Go runtime
// processes. It intentionally has no HTTP, storage, or domain dependencies.
package platform

import "context"

type Role string

const (
	RoleBFF    Role = "bff"
	RoleCore   Role = "core"
	RoleWorker Role = "worker"
)

type Health struct {
	Status string `json:"status"`
	Role   Role   `json:"role"`
}

func Live(role Role) Health  { return Health{Status: "ok", Role: role} }
func Ready(role Role) Health { return Health{Status: "ready", Role: role} }
func Unavailable(role Role) Health {
	return Health{Status: "unavailable", Role: role}
}

type Probe func(context.Context) error

// IsReady runs a dependency probe without exposing its underlying error. A
// nil probe is not ready, because an omitted dependency must never be treated
// as an implicit success.
func IsReady(ctx context.Context, probe Probe) bool {
	if probe == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return probe(ctx) == nil
}
