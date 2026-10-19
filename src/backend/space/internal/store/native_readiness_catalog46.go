package store

// Fixed canonical46 DDL exported from fresh pinned PostgreSQL16/18 cohorts.
// Keep every column/index/constraint; no runtime-learned expected value.
const nativeReadinessStructure46SHA = "92f7340c1946cf4b03ce6d2cd225c8bff26f6ec08b085810877d02c3157ff674"
const nativeReadinessStructure4618SHA = "3185967ae10acff48fa65830ec02a0e69da31e2b9323ee14f8083434b5e63217"

// Each body digest is derived from the actual new46 source, not a live seal.
var nativeReadinessGlobalRemovalPins = []nativeReadinessFunctionPin{
	{"native_membership_guard.bot_global_lock(uuid)", "49e5e9e99304ee53330aa9c93bacc1a76164a89aa8902663afb56b308071b8b1", "plpgsql", "void", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_bot_native_terminal(uuid)", "9ed49d0149c6a8e27ec1e907c45dfd3baa0130c5cab7ffa6b1e071a87ab68d1e", "plpgsql", "boolean", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.bot_global_immutable()", "09fc3b237656f14347c684fcfd2a138f289ffb87802c2c30f6316282e5bd23f1", "plpgsql", "trigger", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.bot_add_terminal_guard()", "597c2d5cd14f1492d336beb3553605a08344a9fd7f5a447864f3d6d56efe062f", "plpgsql", "trigger", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.bot_terminal_registration_guard()", "ab30e292f9839b57c9539ced798910a350c5725737d3d6c3591643c962b7e479", "plpgsql", "trigger", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_bot_global_removal_saved(uuid,uuid)", "4bdd641316beead3f0bb4892146cfd45e5ba0ab2fc6eb4e1fe5389253bb02501", "plpgsql", "bytea", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_bot_global_removal_record(uuid,uuid,bytea,uuid,uuid,uuid,uuid,bytea,bytea,bytea,uuid,bytea,timestamptz)", "13d4b2a58a7d324385388d1bbfc6789984992f21eaf93259d77bee1b41b65eff", "plpgsql", "bytea", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.bot_receipt_terminal_cleanup()", "bef60a8b5724515f820f1261540caa3a76521c1d5956b75ec664492d0e4bca61", "plpgsql", "trigger", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
}

const nativeBotGlobal46MigrationSHA = "f8b645924997788995eb40614e26fde815715a27b95d915517e90e6fc5e0dc09"
