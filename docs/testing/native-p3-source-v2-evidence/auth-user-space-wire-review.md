# Extended current-input generated compatibility audit

Read-only comparison of `fa2eac69f0262409b0ab0f55fd76d200220bc2c7` (H) to `9df1fa83e2cb8741c7fdfc1003658131ccc267b7`. No tests, runtime, tracked source edits, or authority-record changes. Scope extends the sibling Bot/Messaging audit to the specified Auth/User/Space generated context and four module manifests.

Reproduction: `audit_auth_user_space_wire.py` uses the sibling `audit_bot_wire.py` helpers to read exact Git blobs, decode embedded FileDescriptorProto records, compare complete named declaration bytes, and compare generated function text after whitespace/import-alias normalization. `9df-auth-user-space-wire-audit.json` records exact SHA256/Git blobs, candidate classification, dependency changes, all old/new symbol names, field-level differences and new protocol descriptors. This static inspector is evidence about generated wire/code preservation, not a compiler execution or admission proof.

## Existing surface results

| Surface | Auth | User | Space |
|---|---:|---:|---:|
| Old message declarations | 80 identical | 92 identical | 124 identical; 5 extended |
| Old enum declarations | 8 identical | 3 identical | 4 identical |
| Old RPC method descriptors | 37 identical | 40 identical | 53 identical |
| Old getters, alias-normalized | 193 unchanged | 248 unchanged | 340 unchanged |
| Old generated gRPC functions | 113 unchanged | 122 unchanged | 161 unchanged |

No old method, message, enum, field or getter was removed. Complete old method descriptors preserve input/output type names, streaming flags and options. Full-method constants and prior dispatch mappings are unchanged. Package/options/syntax and service non-method descriptor fields are unchanged. The current User-local user `.pb.go` and `_grpc.pb.go` bytes exactly equal the audited Voice-local copies; both new protocol mirror pairs are also identical.

Auth adds only `GetChatSessionEpochFloor` plus its two messages. User adds `CaptureSpaceMemberAccountBindings`, whose DTOs live in the new imported `space_account_capture.proto`; all messages already in user.proto remain identical. Space adds 23 top-level messages, `JoinAdmissionState`, and eight RPCs: `GetMemberAdmissionAuthority`, `StartJoinAttempt`, `GetJoinAttempt`, `SubmitJoinAttempt`, `ListJoinRequests`, `GetJoinRequest`, `ApproveJoinRequest`, `SetEntryPolicy`. These are new exported interfaces, not old authority inherited by default.

## Space extensions require a separate runtime compatibility decision

For each of these five messages, **every old field descriptor is byte-identical**, including its number, label, scalar/message type, type name, JSON name, optional/oneof metadata and options. Non-field message metadata is also identical. The complete message descriptors differ because of these additions:

| Existing message | New fields |
|---|---|
| `BanMemberRequest` | `operation_id` string, field 5 |
| `UnbanMemberRequest` | `operation_id` string, field 3 |
| `AddBotMemberRequest` | `installation_intent` BotInstallationIntent, field 3 |
| `RemoveBotMemberRequest` | `installation_intent` field 3; `install_operation_id` string, field 4 |
| `GetSpaceChatCreationOutcomeRequest` | `public_actor` SpaceChatCreationPublicActor, field 3 |

Thus the old selected field and RPC wire projection is preserved; the complete Space message surface is **not byte-identical**. An old request lacking the new operation/install binding may now be intentionally rejected by a protected owner. This audit makes no assertion that old server authorization, request acceptance, disclosure or mutation behavior is unchanged. Those changes must be covered by the existing native-owner/receiver evidence and explicitly classified by the whole-unit author. In particular, `public_actor` affects result-disclosure qualification; it is not an ordinary visibility grant merely because the added field is optional on the wire.

## Module dependency provenance

All four go.mod diffs are additions only; no prior requirement/replacement/version was altered. The exact diffs are in JSON.

| Changed manifest | Added dependency / provenance |
|---|---|
| User-local `user/go.mod` (old context) | `voice.app/voice/chat v0.0.0`, replacement `../../../../chat/pb/voice/chat`. The generated `user.pb.go` already imported `voice.app/voice/chat/v1` at H and still does at current9df. This closes an already-existing generated import, not a new Chat operation. |
| Voice-local `space/go.mod` (old projection) | `voice.app/voice/role v0.0.0`, replacement `../../../../role/pb/voice/role`. New `member_admission.pb.go` imports Role DTOs for `MembershipBinding` in ordinary/community admission snapshots. |
| Voice-local `calls/go.mod` (old projection) | Same Role dependency marked indirect and same exact replacement. Existing `calls.pb.go` imports Space; Space now imports Role admission DTOs. |
| Voice-local `subscription/go.mod` (old projection) | Same Role dependency marked indirect and same replacement. Existing `subscription.pb.go` imports Space; the same transitive Role closure applies. |

The actual Calls/Subscription generated import files and Role generated member-admission target/module are hashed in JSON. These are protobuf package dependencies and descriptor registration; importing them does not invoke Role or grant a permission. Nevertheless the three go.mod paths are **projection entries**, not context-only generated noise, so changing their hashes needs the whole-unit projection decision. This source audit does not supply that decision.

## New native protocol paths

`voice/user/v1/space_account_capture.proto` and generated Voice/User copies were absent at H. Its request binds protocol, operation, purpose, Space, cohort digest and the complete cohort; responses carry per-entry association/terminal provenance and an owner identity cut. Purpose/outcome/origin enums describe association evidence. They do not establish current eligibility, membership or a general runtime privilege. New source descriptors contain DTOs/enums only, no independent service declaration; the new method lives in UserService. Caller restriction, exact cohort, receipt/history/cut validation and private-listener registration remain native User-owner implementation obligations outside this generated audit.

`voice/space/v1/member_admission.proto` and generated Voice/User copies were absent at H. Imports are Duration, Timestamp, the new common Bot binding and Role member-admission DTOs. Lifecycle/ownership/source/native-publication fields describe a current owner snapshot with generations, typed bindings, retired floors, tree relation and validity remaining; they are not an ACL decision or portable grant. New Bot binding echo is only a selector/result of protected validation, not authorization carried by raw protobuf. Native-kind provenance must be validated by existing owner code; the newly generated enum must not be taken as an independent permission allowlist. The descriptor itself declares no service; SpaceService adds the observer RPC. Source commentary is not proof that runtime registration/caller enforcement matches it.

## Reuse boundary

The nine changed old context entries (Voice Auth/User/Space generated pairs, User generated pair and User go.mod) have the exact preservation/addition evidence above. The three changed old projection manifests have explicit dependency provenance. New protocol paths remain outside the old record and need explicit inclusion/review; they cannot be silently admitted through a broad context glob.

This is sufficient evidence to describe unchanged old selected DTO/RPC/enum **wire semantics**, with the five Space extension exceptions precisely recorded. It is insufficient to relabel all generated source as harmless, reissue historical eight-clause PASS, or qualify current owner/migrator/permission/runtime behavior. No fresh test result is claimed. Component proofs must remain scoped to their tested source identities; the whole-unit author/reviewer retains responsibility for current closures and authority semantics.
