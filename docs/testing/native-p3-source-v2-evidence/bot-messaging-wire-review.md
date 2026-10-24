# Narrow Bot/Messaging generated-context audit

Compared historical H `fa2eac69f0262409b0ab0f55fd76d200220bc2c7` to current `9df1fa83e2cb8741c7fdfc1003658131ccc267b7`. This is a read-only source audit, not a new NativeP3 qualification or a rerun of the historical eight clauses. No tracked source or authority record was edited and no tests were executed for this audit.

Machine evidence: `9df-bot-wire-audit.json`; reproducible stdlib-only inspector: `audit_bot_wire.py` in this directory. The inspector reads exact Git blobs, decodes embedded Go protobuf FileDescriptorProto wire records, compares complete old named declaration bytes and compares existing generated Go functions after whitespace/import-alias normalization. It records SHA256 and Git blob identities, full names, additions, removals and changes. It does not claim a general protobuf compiler or whole-program proof.

## Results limited to the old generated Bot/Messaging surface

| Surface | Bot | Messaging | Result |
|---|---:|---:|---|
| Existing complete message descriptors | 97 | 94 | Byte-identical for every old named message |
| Existing complete enum descriptors | 1 | 7 | Byte-identical; numeric assignments unchanged |
| Existing method descriptors | 43 | 37 | Byte-identical input/output, streaming flags and options |
| Existing generated getters | 191 | 321 | Same signatures/bodies after import alias normalization |
| Existing generated gRPC functions | 131 | 113 | Same signatures/bodies after normalization |
| Existing full method constants | 43 | 37 | Same full RPC strings |
| Existing unary dispatch entries | 42 | 37 | Same method-to-handler mapping (Bot also has its existing stream) |

File descriptor package/options/syntax and service non-method fields are unchanged. Both files add only the import `voice/common/v1/bot_message.proto`. Bot adds `CheckBotMessageSendAuthorityRequest`, `CheckBotMessageSendAuthorityResponse` and RPC `CheckBotMessageSendAuthority`. Messaging adds `SendBotMessageRequest`, `SendBotMessageResponse` and RPC `SendBotMessage`. The new common binding contains eight explicit identity/incarnation fields and no permission/VIEW/role bit. Existing SendMessage fields, existing Bot SendBotMessage fields and their meanings at the wire boundary are unchanged.

This **does add exported generated Go methods and descriptors**. It does not modify an old exported method or prove that every server implementation automatically enforces admission: generated stubs themselves are transport code, not authorization. No old authority enum or selector value was widened. The new runtime routes need their own exact caller/body/owner/native/expiry qualification; they cannot inherit old authority just because the old DTOs are stable.

The four changed Voice-local generated files are classified `context` in the old record by WT48's candidate manifest. `src/backend/voice/pb/voice/common/v1/bot_message.pb.go` is new and outside the old record. `src/backend/voice/go.mod` is byte-identical (SHA256 `c8a4a0e0ebc834ffbca3451ef33711ed80e1a1863a6f61bba30179f2b4b8aa08`): its common replacement still targets `../user/pb/voice/common`, and Messaging replacement still targets `../messaging/pb/voice/messaging`. These Voice-local generated files must not be described as newly direct runtime imports based solely on their presence in the context ledger. An exact-commit `git grep` found no BotService/MessagingService/SendBotMessage/CheckBotMessageSendAuthority reference in non-generated `src/backend/voice` source. Transitive package closure remains the whole-unit owner's responsibility.

## Existing component evidence that can be reused narrowly

The current generated Messaging `.pb.go`/`_grpc.pb.go` files at the actual `../messaging/pb/voice/messaging` replacement and common binding at `../user/pb/voice/common` are byte-identical to their audited Voice-local copies. The user-local Bot generated pair is also byte-identical. JSON records these five exact-copy comparisons and hashes, so the descriptor result is not limited to an unused duplicate while a different compiled replacement silently diverges.

Seven selected Messaging receiver/principal/runtime/test blobs at current9df are exactly equal to published/tested `3552533e282df156d7ee803bf45548d7fb31bbc1`; exact hashes are in the JSON. Prior logs under `D:/Git/Voice/tmp/pr875-repair/messaging-bot-send` are hashed in that JSON: `receiver-pg-m036.log`, `pg-targets.log`, `short-all.log`, `vet-all.log`, `race-focused.log`, and `lint-and-main-final2.log`.

Those logs cover the protected Messaging receiver's actual-JWT/no-VIEW first send, exact replay, changed body/installation conflict, owner revocation before write, timeout, deferred/posted-as-chat/foreign binding denials, expiry during final callback, both text/attachment after-lock hooks, restricted mTLS listener, scoped short/vet/race/lint. They support only that component and its tested dependencies. They do not establish current Voice owner-permission semantics, cross-service Compose acceptance, Bot producer guard correctness, or all NativeP3 clauses. Bot producer/callback evidence belongs to its author; do not infer it from Messaging's proof.

## Qualification boundary and missing proof

The old H eight-PASS record is historical evidence supplied by the whole-unit owner, not re-executed here. This audit supports classifying these four existing generated files as additive compatibility drift for the old DTO/RPC projection. It does **not** justify copying all eight PASS values onto current9df, silently admitting the new common path, or declaring all 554 context/projection entries generated-only.

Whole-unit qualification still requires explicit treatment of the new common generated path and import dependencies, the exact current source/context hashes, and independent assessment of authority source/migrator/runtime changes owned elsewhere. Current R23 descriptor CI reports the additional common import/source projection mismatch; Bot author owns that manifest/oracle repair. That repair is not complete merely because this independent semantic comparison is clean. No authority file modification is authorized or performed by this audit.
