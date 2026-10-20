package store

// Closed49 expectations from the actual pinned migration cohort, never a live fallback.
const nativeReadinessStructure49SHA = "b94dba108c6bbe1829eb7cca52e496ffeec4859393dff694a14c369aa8944ff0"
const nativeReadinessStructure4918SHA = "980eb8ec114210f98c2e440b7a051ac571b0874b6f1ec906b57dc360d366ae86"
const nativePurgeChild49MigrationSHA = "cf0c498727d8540927faf739c63285761ae00e426099dc4616e2e32282b04931"

var nativeReadinessPurgeChild49Pins = []nativeReadinessFunctionPin{
	{"native_membership_guard.bot_purge_child_parent(uuid,uuid,bigint,uuid,bytea,bigint)", "20399688158509c38b723ca37e6c3667972c3587cbfebe4886e798e2f0182795", "sql", "boolean", "s", false, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.bot_purge_child_immutable()", "b0fe0fcdfa78423c168a36ba3ae777e579084ab28f91c0e1ef03a086362f9764", "plpgsql", "trigger", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.bot_purge_child_parent_completed()", "2aee69bcabc091ee33ec393a8df8a6bc0ee26202e56b5440dfdae9f4de8ce6f8", "plpgsql", "trigger", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_bot_purge_child_saved(uuid,uuid,bytea)", "b1cedeb685ac07201bc13bcd7c71fa4cbcbc5cf52550cd51b16a658f380e4c08", "plpgsql", "bytea", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_bot_purge_child_record(uuid,uuid,uuid,bigint,uuid,bytea,bigint,uuid,uuid,uuid,uuid,uuid,bigint,bytea,bytea,uuid,bytea,timestamptz)", "7affb95c9c568fc89dad43a8bd9033109925f901f60202580b62bc4dec6395cb", "plpgsql", "bytea", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_bot_purge_child_expire(uuid)", "00690f0335ff442a655e711227b539cbce0cc5b3682b2a29ab4308357ec18523", "plpgsql", "bigint", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
}
