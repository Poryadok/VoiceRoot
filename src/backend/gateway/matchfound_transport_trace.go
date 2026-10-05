//go:build matchfoundtransportdiag

package main

import (
	"fmt"
	"os"
)

// Set only by the private MatchFound transport fixture when building its exact
// clean source revision. This diagnostic build emits fixed stages only.
var matchFoundTransportTraceRevision string
var matchFoundTransportTraceTree string

func traceMatchFoundTransport(stage string) {
	switch stage {
	case "gateway-fallback", "rest-namespace-not-public", "rest-upstream-missing", "matchmaking-client-missing", "complete-match-decode":
		fmt.Fprintf(os.Stderr, "VOICE_MATCHFOUND_TRACE|%s|%s|%s\n", matchFoundTransportTraceRevision, matchFoundTransportTraceTree, stage)
	}
}
