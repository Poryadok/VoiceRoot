// Code generated from the exact43/44 migration function bodies. DO NOT EDIT.
// Pins authenticate the guard before its protected DB epoch seal is trusted.
package store

var nativeReadinessFunctionPins = []nativeReadinessFunctionPin{
	{"public.guard_join_start_selection_immutable()", "26df627ea0cdb86344cc825cb035cf7bb9f0849c9591fbef7f80a94cf54cf756", "plpgsql", "trigger", "v", false, false, ""},
	{"native_membership_guard.coverage()", "528978a551a23582a9b41a5a363fd8e6e2665bd3438c4cef80884a68e3804c89", "sql", "text[]", "i", false, false, ""},
	{"native_membership_guard.catalog()", "e23a1862746fd405f888bee28110063250d59ee850e5cc0245080f9460f5ee33", "sql", "bytea", "s", false, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.scope(uuid)", "b200580cfba815cfb63fc9d984d6a78318b943b0ec65c260131cfcf6c57e8ea3", "plpgsql", "void", "v", false, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.cohort(uuid)", "42554f74414e54781419358590b4d242d91d69b288bba73163da23594068e8d9", "sql", "uuid", "s", false, true, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.sources(uuid,uuid,timestamptz)", "29d444732b60de9314fccfe03d43d9c4b1296841e6f09e57d1896274754205e4", "sql", "jsonb", "s", false, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.head_sources(jsonb,timestamptz)", "d7ed360c92c4d17e7c6b5c893e13e4b429924dc30380352c3dee1efe239921e9", "sql", "jsonb", "i", false, false, ""},
	{"native_membership_guard.digest_space(uuid)", "cf264a96881d94f693d6e7df622ad9373338382a6e52798c7f532a5fbe80195f", "plpgsql", "bytea", "s", false, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.open_frame(uuid,uuid[],text)", "6b1fef31ca02604cd6fdebce99ec52dc120da4c41f0b36b47f0a7ba12ae05168", "plpgsql", "uuid", "v", false, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_native_membership_begin(uuid,uuid[])", "4fcfe644f24a7c99ae1961c163968543186bcc8bb752657e8c5bcdec36314da6", "plpgsql", "uuid", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_native_membership_profiles(uuid,uuid)", "407d46c95ef4d4827cb6469f95c547f003b7b187fab39477b07a19e888ce4814", "plpgsql", "uuid", "v", true, true, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.terminal(uuid)", "b34d68d29901bd1c46525ec36df2dc6c3ba74c1393aced1e36b2587b46ae21b1", "sql", "boolean", "s", false, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.source_before()", "5e77aa72cbd859ccb1010dcaab53866be9b0647496a54e2a23707ae458dd5b1d", "plpgsql", "trigger", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.source_after()", "45eaf2d4265d87d677297ced19f0b2580577d2ddaff203491abdc58b36206ec7", "plpgsql", "trigger", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.verify_completion(uuid,boolean)", "f5dc10fad739793032332328ccd16ddceeb7d0d33388b6c8593e01743044c44b", "plpgsql", "void", "v", false, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_native_membership_complete(uuid,uuid)", "f726a0872948e15b057a0830555700d8e6f99ff275c8423ef12bc2a30e6e6117", "plpgsql", "void", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.commit_check()", "10d74121b6d28aa6ab90f213e7ca9363c1835ba587e418b630ddb889d8815e82", "plpgsql", "trigger", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_native_membership_require(uuid)", "d3be434b64a84efe159fed97798e6e63891507c2b4ccaaa98e72d09bad44a056", "plpgsql", "void", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_native_membership_initialize(uuid)", "69081b5a6b127671d23bb5ebdd72a478c523ea6bd966d9a2a12bebb734f10e7b", "plpgsql", "void", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"native_membership_guard.terminal_cleanup()", "324a6cf7ab330d45000bfb3532b63e511fd74b2688adbb618830989368dcb75e", "plpgsql", "trigger", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
	{"public.space_native_membership_complete_purge(uuid,uuid)", "72a285e5c774d7e82909fe19ad49a72890826ef69d9b79cae64f4ddeb7dde973", "plpgsql", "void", "v", true, false, "search_path=pg_catalog, public, native_membership_guard, pg_temp"},
}
