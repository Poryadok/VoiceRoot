package store

// PostgreSQL16 canonical structure for exact immutable M45 01d977cd on44.
// This is fixed source metadata, never a live epoch or arbitrary current hash.
const nativeReadinessStructure45SHA = "68d06f1a4d72f971d966132192ce53a321d81998bb854d6c23c6a02e9ea6946a"

var nativeReadinessLegacySelectionPin = nativeReadinessFunctionPin{
	signature: "public.guard_join_legacy_selection_immutable()",
	body:      "27e75bee3ae2309d8e1864255c5ab33aabcb31cf68d820759d85cb36b0ed4088", language: "plpgsql", result: "trigger", volatility: "v",
}
