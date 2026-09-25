#!/usr/bin/env bash
# Keep the live emptyDir-backed source Deployment immutable during candidate
# staging. Optionally preserve the current NATS Service selector for a stale
# clean-install marker when no explicit clean-install opt-in is active.
set -euo pipefail
preserve_nats_service_selector="${VOICE_NATS_PRESERVE_SERVICE_SELECTOR:-false}"
case "${preserve_nats_service_selector}" in
  true|false) ;;
  *) echo 'ERROR: VOICE_NATS_PRESERVE_SERVICE_SELECTOR must be true or false' >&2; exit 1 ;;
esac
awk -v preserve_service="${preserve_nats_service_selector}" '
  function flush_record() {
    if (record != "" && !(kind == "Deployment" && name == "voice-nats") &&
        !(preserve_service == "true" && kind == "Service" && name == "voice-nats")) {
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
    if ($0 ~ /^kind: /) kind = substr($0, 7)
    if (kind == "Deployment" && $0 == "  name: voice-nats") name = "voice-nats"
    if (kind == "Service" && $0 == "  name: voice-nats") name = "voice-nats"
  }
  END { flush_record() }
'
