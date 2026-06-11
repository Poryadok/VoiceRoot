package delivery

import (
	"fmt"
	"strings"
	"time"
)

// ApplyQuietHours suppresses push during DND unless override_mentions allows mentions through.
func ApplyQuietHours(base DeliveryDecision, in DeliveryInput, quiet QuietHoursSnapshot) DeliveryDecision {
	out := base
	if !quiet.Enabled {
		return out
	}
	at := in.At
	if !quiet.At.IsZero() {
		at = quiet.At
	}
	if !inQuietHours(at, quiet) {
		return out
	}
	if in.Type == TypeMention && quiet.OverrideMentions {
		return out
	}
	out.Push = false
	return out
}

func inQuietHours(at time.Time, quiet QuietHoursSnapshot) bool {
	loc := time.UTC
	if quiet.Timezone != "" && quiet.Timezone != "UTC" {
		if l, err := time.LoadLocation(quiet.Timezone); err == nil {
			loc = l
		}
	}
	local := at.In(loc)
	start, ok1 := parseHHMM(quiet.StartTime)
	end, ok2 := parseHHMM(quiet.EndTime)
	if !ok1 || !ok2 {
		return false
	}
	nowMins := local.Hour()*60 + local.Minute()
	startMins := start.Hour()*60 + start.Minute()
	endMins := end.Hour()*60 + end.Minute()
	if startMins <= endMins {
		return nowMins >= startMins && nowMins < endMins
	}
	return nowMins >= startMins || nowMins < endMins
}

func parseHHMM(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return time.Time{}, false
	}
	var h, m int
	if _, err := fmt.Sscanf(parts[0], "%d", &h); err != nil {
		return time.Time{}, false
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &m); err != nil {
		return time.Time{}, false
	}
	return time.Date(2000, 1, 1, h, m, 0, 0, time.UTC), true
}
