package store

import "voice/backend/pkg/authoritysource"

// Exact21 is a different projection contract. It is never served as state1.
// Every current contribution/floor/owner writer participates in the same Role
// epoch; immutable receipt guards protect the witness joined by preparations.
var roleAuthorityCatalog21 = func() authoritysource.Catalog {
	c := roleAuthorityCatalog20
	c.Version = 21
	c.Tables = append(append([]string(nil), c.Tables...), "role_member_source_floors", "role_member_admission_preparations", "role_member_admission_receipts", "role_member_contributions", "role_member_owner_authority")
	c.Triggers = append(append([]authoritysource.CatalogTrigger(nil), c.Triggers...),
		authoritysource.CatalogTrigger{Table: "role_member_source_floors", Name: "role_member_source_floor_change", Function: "role_member_source_floor_guard", Type: 27},
		authoritysource.CatalogTrigger{Table: "role_member_source_floors", Name: "role_member_source_floor_truncate", Function: "role_member_source_floor_guard", Type: 34},
		authoritysource.CatalogTrigger{Table: "role_member_source_floors", Name: "role_authority_revision_member_source_floors", Function: "role_authority_revision_changed", Type: 29, Argument: "space_id"},
		authoritysource.CatalogTrigger{Table: "role_member_admission_preparations", Name: "role_authority_revision_member_preparations", Function: "role_authority_revision_changed", Type: 29, Argument: "space_id"},
		authoritysource.CatalogTrigger{Table: "role_member_admission_preparations", Name: "role_authority_revision_member_preparations_truncate", Function: "role_authority_revision_floor_guard", Type: 34},
		authoritysource.CatalogTrigger{Table: "role_member_admission_receipts", Name: "role_member_admission_receipt_change", Function: "role_member_admission_receipt_immutable", Type: 27},
		authoritysource.CatalogTrigger{Table: "role_member_admission_receipts", Name: "role_member_admission_receipt_truncate", Function: "role_member_admission_receipt_immutable", Type: 34},
		authoritysource.CatalogTrigger{Table: "role_member_contributions", Name: "role_member_contribution_insert", Function: "role_member_contribution_guard", Type: 23},
		authoritysource.CatalogTrigger{Table: "role_member_contributions", Name: "role_member_contribution_truncate", Function: "role_member_contribution_guard", Type: 34},
		authoritysource.CatalogTrigger{Table: "role_member_contributions", Name: "role_authority_revision_member_contributions", Function: "role_authority_revision_changed", Type: 13, Argument: "space_id"},
		authoritysource.CatalogTrigger{Table: "role_member_owner_authority", Name: "role_member_owner_authority_change", Function: "role_member_owner_authority_guard", Type: 31},
		authoritysource.CatalogTrigger{Table: "role_member_owner_authority", Name: "role_member_owner_authority_truncate", Function: "role_member_owner_authority_guard", Type: 34},
		authoritysource.CatalogTrigger{Table: "role_member_owner_authority", Name: "role_authority_revision_member_owner_authority", Function: "role_authority_revision_changed", Type: 29, Argument: "space_id"},
	)
	c.Functions = append(append([]authoritysource.CatalogFunction(nil), c.Functions...),
		authoritysource.CatalogFunction{Name: "role_member_admission_receipt_immutable", Arguments: 0, ReturnType: "trigger", BodySHA256: "9ddc27f44d0eed03f3f8f5d7000397ea1456b1dacf069e123459937c88dbc444"},
		authoritysource.CatalogFunction{Name: "role_member_source_floor_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "854569c5d41901693538ddccfd2143394d0ee834521e2a3c13c2ba2217027ca2"},
		authoritysource.CatalogFunction{Name: "role_member_contribution_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "80ac96acb27904fed0c4af7d8ae657098492ad30e2ad8cc2fae67dac4906ac4c"},
		authoritysource.CatalogFunction{Name: "role_member_owner_authority_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "3a876182c5497394827048ba06f56ee547cfdbc18b38021007837a48788fb63b"},
	)
	return c
}()
