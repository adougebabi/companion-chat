package browser

import "context"

type health struct {
	Status string `json:"status"`
	Role   string `json:"role"`
}

func liveHealth() health        { return health{Status: "ok", Role: "api"} }
func readyHealth() health       { return health{Status: "ready", Role: "api"} }
func unavailableHealth() health { return health{Status: "unavailable", Role: "api"} }

func isReady(ctx context.Context, probe func(context.Context) error) bool {
	if probe == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return probe(ctx) == nil
}
