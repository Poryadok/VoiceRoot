# A1 lifecycle overlay listener collision

Existing task: A1-SPACE-CREATOR-COMPOSE-UNAVAILABLE, owner Рудеус-342.
Base f1b67e965c833469c88d535bbfe4a363b37c183c. Fresh lease27,
18ca1f4264832c5ae1183c6ffcec2749. User excludes CodeGraph until recovery.

The opt-in local lifecycle overlay layers after the media overlay, but both
Space private listeners currently bind :9094. Preserve the media listener and
all activation settings. Move only Gateway lifecycle listener/expose and its
Gateway client target to9095. This does not connect the overlay to default CI,
enable native45 or close canonical POSTspaces503.

Acceptance: render actual base+media+lifecycle Compose; assert distinct9094/9095,
matching Gateway target, retained read-only TLS mounts and unchanged opt-in
activation. Add the contract to existing phase0-fixture-test/ci-script-tests.

Before implementation, add the regression and attempt private Linux execution.
Docker Desktop Linux engine pipe is currently missing. No host test executable,
new listener, DEV/STAGING or hosted CI invocation is allowed. Record validation
as unavailable until actual Linux evidence exists; no RED/GREEN claim from text.

Implementation now changes only the three lifecycle port values to9095 and
adds a collision comment. Voice communication9094 and all activation flags are
unchanged. Both existing Docker endpoints fail because their named pipes are
absent; only Docker's service WSL distro exists, not an approved general Linux
runner. No alternative runtime is borrowed. Actual RED and GREEN execution
remain required before integration; preserve basef1b for causal RED replay.

Full Creator assembly remains gated on accepted468 producer→186 native closure
and current Role/authority inputs. These are separate from this port repair.
