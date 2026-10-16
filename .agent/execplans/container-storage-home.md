# ExecPlan: move shared-host container storage to /home

## Purpose

User explicitly requests all container storage on /home before continuing dev,
and proof that TTenrec data survives container recreation. Existing /var is only
93 GiB; /home has 318 GiB free on the same physical 1 TB disk.

## Context

Read docs/STAGING_SERVER.md, DEPLOYMENT.md, OPERATIONS.md, CONTRIBUTING.md,
TESTING.md, installed ttenrec skill and independent storage_review findings.
K3s containerd uses39 GiB; Docker9 GiB; NATS preservation8 GiB; journal4 GiB.
All17 PVs use local paths under /var/lib/rancher/k3s/storage. TTenrec SQLite
tracker.db and WAL are in tracker-data PVC mounted /data. PVC UID is
adbbd51a-a407-496b-beb7-364bfefc26d5; its PV Delete reclaim makes PVC deletion
unsafe, while pod recreation preserves data. Never delete/recreate PVCs.

## Scope

Move data backing k3s, Docker, kubelet, system containerd, container logs and
NATS preservation to root-owned /home/container-data. Bind original absolute
paths to preserve PV references, service config and NATS custody paths.
Preserve Docker bind mounts already under /home. No repartition/resize,
credential rotation, application/schema changes or data cleanup.

## Milestones

- [x] Read-only disk/PV/TTenrec persistence audit and independent safety review.
- [x] Consistent TTenrec backup and checks; inventory all running workloads/PVs.
- [x] Root migration executor prepared, reviewed, bounded and independent of Docker.
- [x] Graceful workload shutdown, no-shim/no-submount proof, cold copy+validation.
- [x] Persistent binds and fail-closed startup dependencies; restart and verify.
- [x] TTenrec pod recreation with same DB facts; all prior workloads recover.
- [x] Reclaim original /var duplicates only after verified backup on /home.

## Detailed Steps

1. Capture before inventory and SQLite backup/quick_check/table counts via
   sqlite3 read-only URI+backup API. Keep outputs private; print aggregates only.
2. Install rsync from Debian packages if absent. Run a host systemd service
   independent of Docker and in host mount namespace; user authorized downtime.
3. Stop controller-managed apps gracefully (record replica counts), then CRI
   stop remaining sandbox containers, stop k3s and Docker/socket+containerd.
   KillMode=process means stopping daemons alone is insufficient. Do not use
   k3s-killall.sh indiscriminately; it rewrites networking and removes paths.
4. Prove quiescence (runtime processes and source nested mounts absent), cold
   rsync preserving numeric owners, ACL/xattrs/hardlinks; compare dry-run checksum.
5. Keep old dirs as rollback copies. Install bind mount units ordered on /home
   and /var, require mounts for runtimes, verify mounted source FS before start.
6. Restore recorded replicas, verify same PVC/PV UIDs, DB integrity/counts,
   basic endpoints and workload readiness. Explicitly recreate TTenrec pod.
7. Copy old dirs to root-only /home rollback; checksum verify before removing
   exact allowlisted old /var paths. Retain rollback until later owner cleanup.
8. Update operator docs and return to dev rollout.

## Validation

Before/after hashes of PV identities, counts/health, SQLite integrity/table
counts, device/mount provenance, private migration checksums, persistent units
and dependency guards. No secret or task text in logs.

## Progress

- [x] User authorized migration and TTenrec check, pending dev preserved.
- [x] Auditor found service KillMode process, shared mounts, static local PVs.
- [x] Backups and root executor preparation.

## Decisions

Bind mounts keep existing paths and private custody valid; runtime data-dir
changes or PV recreation would introduce avoidable configuration drift.
Root operations run through existing authorized Docker access, with a detached
host systemd executor so stopping Docker cannot kill migration.

## Risks And Follow-Ups

All apps on this single host are briefly unavailable during cold cutover.
Same physical disk is not off-host backup. Crash recovery and startup readiness
must be proved before deleting any /var duplicate. No full reboot is assumed
until persistent mount ordering is checked.

## Outcome (2026-10-16)

Cold copy and checksum verification completed for all six roots. Persistent bind
units and runtime RequiresMountsFor guards installed. All original 17 PV/PVC
identities survived. All prior workload readiness restored, including canonical
User/Space cold-start recovery. Forced CRI stop of tracker restarted the same
persistent installation; SQLite integrity and full content hashes of both tables
matched. Rollback copies moved to /home and checksum verified before deleting
allowlisted offline /var duplicates. Stage COMPLETE. /var 82 GiB free; /home
199 GiB free after dev builds. Root-only evidence under /home/container-data/migration.
No full host reboot or off-host disaster recovery was tested. Four focused tests pass.
