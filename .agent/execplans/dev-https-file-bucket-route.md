# DEV File HTTPS companion qualification

The consumer is `DEV-0D75-FILE-ADOPTION`; configuration bug
`DEV-HTTPS-FILE-BUCKET-ROUTE` was found during its actual File-only DEV check.
Product source0d75/image8ee3 remains immutable and separate from configuration
source. The first File migration completed5→13 clean, but its returned HTTPS
PUT hit Gateway404. The old Filef358 image was safely restored; schema13 and
one owned pending File/blob remain. DEV was safely released130/gen32.

## Earned evidence and source decision

- Actual signing endpoint is the existing HTTPS API host; its HTTP ingress
  already has the File bucket route, while its HTTPS ingress has only Gateway.
- Owned File80b24a64 was reconciled pending_upload; authenticated S3 HEAD404
  proves object absence. No confirm or repeated upload was made.
- Actual controller is Traefik3.6.6, Kubernetes Ingress classtraefik. Its versioned
  provider source keys routers by namespace/Ingress name/host/path. Default
  rule priority favors the specific bucket rules over the same-host root.
- strictPrefixMatching is false; use Exact bucket root and Prefix bucket-slash
  so sibling names remain outside the route.
- Namespace authority and server dry-run accepted the separate fixed companion.
- Canonical certificate renewal publishes its ten fixed Ingress names and
  fixed Secret, with no list/prune action. The companion is outside selection.
  A focused renewal test keeps that separation explicit.

The source fix is a separate render-only companion module. Certificate issuer
source and protected installed operator bytes remain unchanged. It reuses the
existing exact host/Secret/class/entrypoint, without rewriting signed paths.

Controller references: [versioned provider source](https://github.com/traefik/traefik/blob/v3.6.6/pkg/provider/kubernetes/ingress/kubernetes.go),
[strict-prefix defaults](https://doc.traefik.io/traefik/v3.6/reference/install-configuration/providers/kubernetes/kubernetes-ingress/)
and [rule priority](https://doc.traefik.io/traefik/v3.6/reference/routing-configuration/http/routing/rules-and-priority/).
The separately bounded immutable-image probe passed eight rule/host cases with
networknone and its owned container removed. It uses the file provider to test
the actual binary's predicates/priority; Kubernetes adapter coexistence remains
source-reviewed until the live configuration window, and TLS/renewal were not
exercised by that fixture.

## Remaining execution checkpoints

1. Independent exact-source review and hosted CI, then normal merge.
2. Freeze configuration source/manifest and a reviewed immutable live operation;
   inspect fresh actual49pins, Filef358/schema13, pending owned object and all
   existing Ingress/TLS metadata. Acquire DEV atomically only for execution.
3. Create only the previously absent owned companion under actual TT and
   updater locks; no existing issuer/Ingress/Secret/application/store changes.
4. Prove actual controller/TLS/backend routing and preserved API/other hosts.
   Reconcile pending upload state/object absence before any reviewed retry.
5. Qualify the immutable File artifact in a separately explicit image-only
   package if needed; preserve existing13 and owned data. No new build/relabel.
6. On failure, remove only the exact owned companion UID/resourceVersion and
   recover the image separately if changed; preserve all other resources/data.
   Settle actual operations, observe and safely release the resource.

## Limits

The root-only installed renewal copy was unreadable by the DEV actor. Source
no-prune evidence and an actual installed renewal exercise are distinct.
No live companion/controller/public signature proof or renewal-persistence
PASS follows from source tests/server dry-run. Do not call the old root-copy
patch delivered or use its stale preview CAS. Full-batch same-candidate DEV,
exact-source CI/topology/data and actual stable STAGING promotion interval
remain separate gates; no STAGING mutation is authorized by this plan.
