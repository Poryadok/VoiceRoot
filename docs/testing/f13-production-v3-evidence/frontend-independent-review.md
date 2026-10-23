# Independent Flutter invite client review

Verdict: SOURCE CLEAR for the complete three-file fix. Reviewer /root/role_admission, existing model/effort; no source authorship or reviewer runtime execution for this slice.

Author f81e989de096c1af20d7f21152a0a018a19a8822; integrated root H73531c1e48d63fb100ab279930c98f67c9e97241. Exact Git comparison of all three paths author→H is empty. All entries mode100644/typeblob:

* src/frontend/lib/backend/spaces_client.dart: 9bc6cb854ff873900faa9cab993b4b6b12462d12
* src/frontend/test/spaces_client_invites_test.dart: eee0abd1abe664382e98e08b377f6ad5f7d84ab4
* src/frontend/test/support/bot_live_harness.dart: 16317fc30f044b4c8b34075d106b280aa106879c

Gateway serveSpaceJoin rejects any nonempty HTTP body and constructs invite Code from the route. Existing GatewayHttpClient.postProto already supports an omitted body while decoding a typed protobuf response. Production delta removes only JoinByInviteRequest body argument; route, authorization, JoinByInviteResponse.create and membership mapping remain. No Gateway receiver relaxation, new transport fallback, Role/Space production mutation, or change to pinned Role closure.

Regression captures the actual http.Request and asserts POST, authorization, exact route and empty bodyBytes; it also verifies decoded Space and profile. This meaningfully discriminates the reported RED17-byte {"code":"secret"} request. Harness delta only adds HTTP status/error code/message to a failed join assertion; successful membership Space/profile assertions remain mandatory. No failure is converted to success or skipped.

Author reported causal RED (17 request bytes), GREEN16 invite/tree unit tests and scoped analyzer0issues. These are component validation reports, not independent reviewer execution. Exact author log receipts are supplied separately by /root/bot_installation and should be hash-bound into R2 by its author; this review does not invent their hashes. No hosted acceptance was available at review time. Historical H1/H2 failure facts and old AUTHOR restrictions remain unchanged; this document grants no generic future-source/runtime/activation authority.
