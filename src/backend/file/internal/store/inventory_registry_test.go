package store

import (
	"testing"

	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
)

func TestInventoryRegistryCommitsServiceOwnerAndWriterDimensions(t *testing.T) {
	base := []inventoryCoverageComponent{{Service: "story", Component: "story.media.v1", Owners: []commonv1.ReferenceInventoryOwnerType{commonv1.ReferenceInventoryOwnerType_REFERENCE_INVENTORY_OWNER_TYPE_STORY}, WriterVersion: 1, KeyShapeKnown: true}}
	original := inventoryRegistryEncoding(base)
	for _, field := range []string{"service", "component", "owners", "writer", "shape"} {
		t.Run(field, func(t *testing.T) {
			changed := append([]inventoryCoverageComponent(nil), base...)
			switch field {
			case "service":
				changed[0].Service = "user"
			case "component":
				changed[0].Component = "story.other.v1"
			case "owners":
				changed[0].Owners = []commonv1.ReferenceInventoryOwnerType{commonv1.ReferenceInventoryOwnerType_REFERENCE_INVENTORY_OWNER_TYPE_MESSAGE}
			case "writer":
				changed[0].WriterVersion = 2
			case "shape":
				changed[0].KeyShapeKnown = false
			}
			require.NotEqual(t, original, inventoryRegistryEncoding(changed), "a component-name-only digest is not the accepted coverage registry")
		})
	}
}

func TestInventoryRegistryKeepsUnknownKeyShapesExplicit(t *testing.T) {
	covered := map[commonv1.ReferenceInventoryOwnerType]bool{}
	components := map[string]bool{}
	unknown := 0
	for _, c := range requiredInventoryRegistry {
		require.False(t, components[c.Component])
		components[c.Component] = true
		require.EqualValues(t, 1, c.WriterVersion)
		if !c.KeyShapeKnown {
			unknown++
			continue
		}
		require.NotEmpty(t, c.Service)
		require.NotEmpty(t, c.Owners)
		for _, owner := range c.Owners {
			require.True(t, owner >= 1 && owner <= 8)
			covered[owner] = true
		}
	}
	for owner := commonv1.ReferenceInventoryOwnerType(1); owner <= 8; owner++ {
		require.True(t, covered[owner], "closed enum coverage is necessary but not sufficient")
	}
	require.GreaterOrEqual(t, unknown, 2, "catalog retention and pending-write key shapes are not inferred from names")
}
