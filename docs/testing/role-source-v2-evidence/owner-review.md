# Role historical correction applicability at84a

Target84a354f191b19cb5a95d17a8831fc4885a2713de. Old record ce229477510f96c9ac66f2bb5bec69ac1b020430 remains immutable. This is a bounded source applicability audit, not generic duplicate authority, historical runtime clearance, or whole-current-Role acceptance. I authored parts of6313; final independent review remains f13_review's responsibility.

Exact eleven old/current blobs and current SHA256 are in f13-role-correction-84a-inventory.json. Eight drift, three unchanged. Git confirms required original correction2ce88a9892989686cd4fd5734e8b414a3c914564 is an ancestor. Entire src/backend/role is byte-identical6313cc6e5→84a.

## Preserved correction

ServerOptions still calls StrictUnaryInterceptor directly after the listener's method gate. There is no redundant post-Strict nickname-only wrapper. The listener now additionally recognizes guarded Bot removal, Space reads and enabled delegated Role methods; those do not wrap or replace creation/nickname dispatch.

creation_decision.go and nickname.go are byte-identical to the old eleven-file record. Strict still reads transport, hashes full protobuf, verifies signed principal, then branches exact MEMBER_MANAGE_NICKNAMES to RequireNicknamePermission and exact creation to validateCreationDecision. Newly explicit workload/Chat/Space/Bot selectors are separate earlier branches; creation/nickname do not select workload or Chat verifiers. Existing pure service:space/audience role/RPC/body binding, canonical IDs, no chat/voice scope predicates remain. Runtime VerifyService still performs signature/JTI replay validation; the optional Bot verifier does not authorize a Space token and the selector predicate rejects a Bot principal for these names.

## Eight changed entries

* role-service.md: documents native admission/qualified decisions and expanded explicitly protected callers. It is not the old catalog20-only documentation delta; successor must bind current doc rather than claim literal old criterion.
* go.mod: adds only local Auth proto module require/replace for current session-floor client; go.sum unchanged.
* ordinary_scope.go: replaces raw WithinSpaces composition with withNativeOrdinary and propagates observed call context/target profiles; CheckPermission gains exact protected workload/Chat checks and post-return credential expiry checks. The handler-level nickname guard remains before ordinarySpaceRPC; the exact creation predicate belongs to Strict on the protected listener. Native path requires verified principal/current Space observation; no new raw bypass is introduced by the correction.
* interceptor.go: exact additional decision selectors and method guards, including ordinary listener denial for their protected routes. Creation/nickname terminal predicate branches remain. These added capabilities are outside the old correction's authority and are supported only by their separate component review/proof.
* listener.go: expanded method gate; direct Strict chain retained.
* runtime.go: current delegated/Auth checker, Bot/admission bootstrap dispatch, optional mTLS JWKS client identity and exact Chat/workload issuer verifiers. TLS server still requires verified client certificate; resolver/replay dependency failures remain closed. No nickname-only post-validation handler.
* creation_listener_test.go and nickname_listener_test.go: each changes only other_permission negative from SPACE_MANAGE_SETTINGS to SPACE_MANAGE_ROLES because SETTINGS now has its own legitimate protected decision. Valid signed request reaches handler once; replay, wrong issuer/hash, raw actor, duplicate credential and empty chat/voice negatives remain. Creation canonical-ID negatives remain. Existing TLS listener fixture and assertion counts are retained.

## Evidence and limits

Original correction proof tmp/swarm/master-intake/role-creation-correction-proof.json SHA25671123a7b85faa460ce87848f0df582da3d4a4dafe90d032d9e217cc22662b706 binds2ce: genuine fresh creation RED then11events GREEN,210 affected events, actual TLS/JWKS/RSA/Redis, vet/lint. Original record's c70/c74/c75 review remains historical;144c69 HOLD is not retroactively changed.

Current component source6313 equals84a for the entire Role directory. Session tmp/pr875-repair/role-admission/role-s2s-closure.md records full principalgrpc/principalruntime/grpcsvc short suites and corresponding vet PASS after final changes; principalruntime includes both actual listener tests above. Separate independent f13 source review was CLEAR for final workload/MM slice. Actual native25+Space51 PASS69.513s is retained at wt47-workload-mm-native-role25.log and supports the changed qualified decision path; it is not a substitute for creation/nickname listener proof or a historical cross-merge run.

No fresh tests executed in this read-only audit. Later shared/generated dependency bytes are not automatically the original6313 test binary; evidence is unchanged Role component applicability, with successor whole-unit dependency qualification owned by the F13 author/reviewer. The successor must bind all11 current entries and any necessary exact dependency closure, preserve original row/taint flags and historical144c69 HOLD, and retain negative current-drift/removed-correction/partial-binding cases. No blanket Role, Native, current/future path or runtime waiver follows.
