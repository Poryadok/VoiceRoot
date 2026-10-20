package store

// Closed48 expectations are compiled from the controlled migration cohort.
const nativeReadinessStructure48SHA = "d851001baa42957a6f7753b63ed5dc0266bff1722ea08fa602f552f820f649c5"
const nativeReadinessStructure4818SHA = "bb7acfb9e045b9a92f80e1961553078497e15964f73dd23985201d453e2a4436"
const nativeMinimal48MigrationSHA = "b528bfb4ea0de835ce1f0eeab46a961a7e8142338bcff741f3a7361550ee9d55"

var nativeReadinessMinimal48Pins = []nativeReadinessFunctionPin{
	{"native_membership_guard.bot_terminal_scope()", "f20c2acdb6d843828b5c4baab3f15056e6053f4c504896794a927e6847577fc1", "plpgsql", "void", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.bot_terminal_minimal_record(uuid,uuid)", "10df2d30264fa18816a04cdc6eee44df69d7001d5495fc603c8b6e13375505f4", "plpgsql", "native_membership_guard.bot_terminal_registrations", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_bot_global_terminal_register(uuid,uuid)", "a236869c29e61dd6a2db07cc1950a3a7e1784892018f3c9d106193b9dfcf0825", "plpgsql", "record", "v", true, true, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_bot_global_removal_saved48(uuid,uuid)", "267b7f6bffd516bf2780db56c46fdc897c046b34ce0c4de7aca0f77ae7d782c7", "plpgsql", "bytea", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_bot_global_removal_record48(uuid,uuid,bytea,uuid,uuid,uuid,uuid,bytea,bytea,bytea,uuid,bytea,timestamptz)", "d43cf49cbab23d7cfe2cfe88ab0a80ca083be0ffc31ac14dacb037074cb0df04", "plpgsql", "BYTEA", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.bot_minimal48_immutable()", "0a1b71c16e6b2e595b8e8b0889572a309b65420c843d676343a80c31f3764408", "plpgsql", "trigger", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.bot_receipt48_terminal_cleanup()", "be4a9d1725dc758b68c2af49b7688105de6ef8e1c073d1882fbb0022f6fdd930", "plpgsql", "trigger", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_bot_native_terminal(uuid)", "388593f95b7595114109f208a53f64ae0b4994de6158bdcff2ddbaeb4b7c3f06", "plpgsql", "BOOLEAN", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.bot_add_terminal_guard()", "b4f5f5f4e39f5bbe32a9b930102a8a619332b2603b602de637c263ac05606964", "plpgsql", "trigger", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_bot_global_removal_record(uuid,uuid,bytea,uuid,uuid,uuid,uuid,bytea,bytea,bytea,uuid,bytea,timestamptz)", "70174cf7f8f0efe4778232e4e7e2e7656a691407206056ccae8d483fd701fcce", "plpgsql", "BYTEA", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
}
