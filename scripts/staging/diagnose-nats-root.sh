#!/usr/bin/env bash
# Read-only, bounded incident diagnostics on the trusted staging runner.
set +x
set -euo pipefail

ns=voice-staging
kubectl get configmap voice-nats-generation -n "$ns" -o json |
  jq -c '{marker:{phase:.data.phase,generation:.data.generation,previousGeneration:.data.previousGeneration}}'
kubectl get deployment voice-nats-pvc-candidate -n "$ns" -o json |
  jq -c '{hub:{replicas:.spec.replicas,ready:(.status.readyReplicas // 0),generation:.spec.template.metadata.annotations["voice.io/nats-generation"],volumes:[.spec.template.spec.volumes[]? | select(.name == "jsdata" or .name == "nats-operator-jwt" or .name == "nats-hub-tls") | {name,pvc:.persistentVolumeClaim.claimName,secret:.secret.secretName}]}}'
kubectl get deployments -n "$ns" -o json |
  jq -c '[.items[]? | select(any(.spec.template.spec.volumes[]?; .name == "nats-service-creds")) | {name:.metadata.name,replicas:.spec.replicas,ready:(.status.readyReplicas // 0),generation:.spec.template.metadata.annotations["voice.io/nats-generation"],serviceCredential:([.spec.template.spec.volumes[]? | select(.name == "nats-service-creds") | .secret.secretName] | first),hubTLS:([.spec.template.spec.volumes[]? | select(.name == "nats-hub-tls") | .secret.secretName] | first)}]'
kubectl get service voice-nats -n "$ns" -o json |
  jq -c '{service:{selector:.spec.selector,clientPorts:[.spec.ports[]? | select(.port == 4222) | {name,port,targetPort}]}}'

classify_log() {
  local log="$1" category=OTHER
  if [[ -z "$log" ]]; then category=EMPTY
  elif grep -Eqi 'incompatible configuration for stream' <<<"$log"; then category=INCOMPATIBLE_STREAM
  elif grep -Eqi 'incompatible consumer' <<<"$log"; then category=INCOMPATIBLE_CONSUMER
  elif grep -Eqi 'permission|authorization|denied' <<<"$log"; then category=PERMISSION
  elif grep -Eqi 'authentication|credential|jwt' <<<"$log"; then category=AUTH
  elif grep -Eqi 'tls|certificate|x509' <<<"$log"; then category=TLS
  elif grep -Eqi 'connection|connect|timeout|timed out|no responders' <<<"$log"; then category=CONNECTIVITY
  elif grep -Eqi 'not found|does not exist' <<<"$log"; then category=NOT_FOUND
  elif grep -Eqi 'bootstrap consumer CREATE failed' <<<"$log"; then category=CONSUMER_CREATE_FAILURE
  elif grep -Eqi 'passed|completed|success' <<<"$log"; then category=PASS
  fi
  printf '%s' "$category"
}

for name in voice-nats-realtime-bootstrap voice-nats-notification-bootstrap voice-nats-search-bootstrap voice-nats-analytics-chat-bootstrap voice-nats-realtime-permissions-preflight; do
  if kubectl get job "$name" -n "$ns" -o json 2>/dev/null |
    jq -c --arg name "$name" 'def safe_reason: if . == "DeadlineExceeded" or . == "BackoffLimitExceeded" or . == "CompletionsReached" or . == "JobSuspended" or . == "PodFailurePolicy" then . else "OTHER" end; {job:$name,generation:.metadata.annotations["voice.io/nats-generation"],active:(.status.active // 0),succeeded:(.status.succeeded // 0),failed:(.status.failed // 0),conditions:[.status.conditions[]? | {type,status,reason:(.reason | safe_reason)}]}'; then
    pods="$(kubectl get pods -n "$ns" -l "job-name=$name" -o json)"
    printf '%s' "$pods" |
      jq -c --arg job "$name" 'def safe_reason: if . == "CrashLoopBackOff" or . == "ImagePullBackOff" or . == "ErrImagePull" or . == "CreateContainerConfigError" or . == "ContainerCreating" or . == "RunContainerError" or . == "InvalidImageName" or . == "CreateContainerError" or . == "PodInitializing" or . == "Error" or . == "Completed" or . == "OOMKilled" or . == "ContainerCannotRun" or . == "ContainersNotReady" or . == "PodCompleted" or . == "PodFailed" or . == "Unschedulable" then . else "OTHER" end; [.items[]? | {job:$job,pod:.metadata.name,phase:.status.phase,conditions:[.status.conditions[]? | {type,status,reason:(.reason | safe_reason)}],containers:[.status.containerStatuses[]? | {name,ready,restarts:.restartCount,waiting:(.state.waiting.reason | safe_reason),terminated:(.state.terminated.reason | safe_reason),exitCode:.state.terminated.exitCode,lastTermination:(.lastState.terminated.reason | safe_reason),lastExitCode:.lastState.terminated.exitCode}],initContainers:[.status.initContainerStatuses[]? | {name,ready,restarts:.restartCount,waiting:(.state.waiting.reason | safe_reason),terminated:(.state.terminated.reason | safe_reason),exitCode:.state.terminated.exitCode,lastTermination:(.lastState.terminated.reason | safe_reason),lastExitCode:.lastState.terminated.exitCode}]}]'
    while IFS= read -r pod; do
      [[ "$pod" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]] || continue
      if events="$(kubectl get events -n "$ns" --field-selector "involvedObject.name=$pod" -o json 2>/dev/null)"; then
        printf '%s' "$events" | jq -c --arg pod "$pod" '{pod:$pod,eventReasons:[.items[]? | {reason:(.reason | if . == "BackOff" or . == "FailedMount" or . == "FailedScheduling" or . == "Unhealthy" or . == "Pulling" or . == "Pulled" or . == "Failed" or . == "Created" or . == "Started" or . == "Killing" or . == "FailedCreatePodSandBox" or . == "FailedAttachVolume" or . == "SuccessfulCreate" then . else "OTHER" end),count:(.count // 1)}]}'
      else
        printf 'NATS_DIAGNOSTIC_EVENTS=%s:UNAVAILABLE\n' "$pod"
      fi
    done < <(printf '%s' "$pods" | jq -r '.items[]?.metadata.name')
    container=bootstrap
    if [[ "$name" == voice-nats-realtime-permissions-preflight ]]; then container=realtime-preflight; fi
    log="$(kubectl logs "job/$name" -n "$ns" -c "$container" --tail=40 --limit-bytes=4096 2>/dev/null || true)"
    printf 'NATS_DIAGNOSTIC_LOG=%s:%s\n' "$name" "$(classify_log "$log")"
    if [[ "$name" == voice-nats-realtime-permissions-preflight ]]; then
      log="$(kubectl logs "job/$name" -n "$ns" -c nats-leaf --tail=40 --limit-bytes=4096 2>/dev/null || true)"
      printf 'NATS_DIAGNOSTIC_LOG=%s:nats-leaf:%s\n' "$name" "$(classify_log "$log")"
    fi
  else
    printf 'NATS_DIAGNOSTIC_JOB=%s:ABSENT\n' "$name"
  fi
done
