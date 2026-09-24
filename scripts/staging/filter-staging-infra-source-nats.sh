#!/usr/bin/env bash
# Keep the live emptyDir-backed source Deployment immutable during candidate
# staging. All other resources, including the candidate, pass through unchanged.
set -euo pipefail
awk '
  function flush_record() {
    if (record != "" && !(kind == "Deployment" && name == "voice-nats")) {
      printf "%s", record
    }
  }
  /^apiVersion:/ {
    flush_record()
    record = $0 ORS
    kind = ""
    name = ""
    next
  }
  {
    record = record $0 ORS
    if ($0 == "kind: Deployment") kind = "Deployment"
    if (kind == "Deployment" && $0 == "  name: voice-nats") name = "voice-nats"
  }
  END { flush_record() }
'
