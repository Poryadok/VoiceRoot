package store

// PostgreSQL18 records the same enforced NOT NULL columns as named pg_constraint
// rows as well as pg_attribute.attnotnull. Keep those additional facts in the
// closed structural projection, with fixed pins from the actual immutable
// migration cohort. No live observation is accepted as a new expected value.
const (
	nativeReadinessStructure18SHA   = "090ecd37160c0c8ef0105d951e1eb0f7d5c209b60653f6f7b386eb3b64aa3d7d"
	nativeReadinessStructure4518SHA = "6aa303a8d921898c93b89c1b6ebf16090456a0ca5d29e3fb83c38a4ab369fa5e"
)

func nativeReadinessExpectedStructure(schema int64, serverVersion int) (string, error) {
	switch serverVersion / 10000 {
	case 16:
		switch schema {
		case 44:
			return nativeReadinessStructureSHA, nil
		case 45:
			return nativeReadinessStructure45SHA, nil
		case 46:
			return nativeReadinessStructure46SHA, nil
		case 47:
			return nativeReadinessStructure47SHA, nil
		case 48:
			return nativeReadinessStructure48SHA, nil
		}
	case 18:
		switch schema {
		case 44:
			return nativeReadinessStructure18SHA, nil
		case 45:
			return nativeReadinessStructure4518SHA, nil
		case 46:
			return nativeReadinessStructure4618SHA, nil
		case 47:
			return nativeReadinessStructure4718SHA, nil
		case 48:
			return nativeReadinessStructure4818SHA, nil
		}
	}
	return "", ErrMemberAuthorityUnavailable
}
