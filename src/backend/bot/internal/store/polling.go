package store

import (
	"os"
	"strings"
)

// DevPollingEnabled is an explicit local-development escape hatch. PollEvents
// has no recipient ACK and cannot be used for reliable production commands.
func DevPollingEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("BOT_ENABLE_DEV_POLLING")), "true")
}
