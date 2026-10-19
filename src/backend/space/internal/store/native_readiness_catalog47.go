package store

// Closed source-derived47 functions and full canonical PG16/18 structures.
// Expected values never come from the serving database's retained/live seal.
const nativeReadinessStructure47SHA = "a3ccb0bcdcc88002867d7e2c1e9f4c6d44015f176afd4bcc531fce74e831484d"
const nativeReadinessStructure4718SHA = "d74afc2c7a2ed5235f2c7c9b8eb8c128168451bc76a6409802b3fe172e418c7f"
const nativeTerminal47MigrationSHA = "8ce59af43a08bc9bdea94fc004ff86c615ec4691f501f19a5591bafd566e937d"

var nativeReadinessTerminal47Pins = []nativeReadinessFunctionPin{
	{"native_membership_guard.terminal_serving_scope(uuid)", "154bbb6d91e4df269381cf5d6770d486640bd42e7d1934cc7c0b81d4691bee56", "plpgsql", "void", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.terminal_representation(uuid)", "fb7325cd8ed0fc51beeafb1c0c622e07df4b7e1cfc80b85924c7ce90239ac45f", "plpgsql", "text", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.terminal_varint(bigint)", "b43966d77457f7f0376165865de0bd010c705d93a91d0858184a7df88a986a14", "plpgsql", "bytea", "i", false, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.terminal_wire_bytes(integer,bytea)", "57a2cdbdb49d97cfdb38531546b8c6f7c49b51a42f2e41ec3f78479e2688ed76", "sql", "bytea", "i", false, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.terminal_event_wire(public.space_lifecycle_outbox)", "8211ba8423806f4fbf4648d8880dff42039b34cfe864d5de74b90a82be51e9f8", "plpgsql", "bytea", "i", false, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_native_terminal_event_delivered(uuid,uuid,bytea,bytea)", "78e01008c9f356aa0bd347121860298a3f996d82659263077af09b44312bbd46", "plpgsql", "void", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_native_terminal_compact(uuid)", "7a042830a95fddaa487229c4f58253c9c877bd3134039b2765b57227df1ade0b", "plpgsql", "boolean", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_native_terminal_expire_events(uuid)", "0dd3d7a0be7168c2f8fd73cb671827fa87d24d55538620547ae3099770b74046", "plpgsql", "bigint", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_native_terminal_expire(uuid)", "b3396a9ab6d51d1247e8070b37c4570ffe8859b32d855371fd053771c194f19b", "plpgsql", "boolean", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.source_before()", "13d23848843c13fe14209da83610b9e3dd7a2d01d4118019b1cef927b5b6e486", "plpgsql", "trigger", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
}
