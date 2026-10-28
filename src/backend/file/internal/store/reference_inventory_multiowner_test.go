package store

import (
	"testing"

	commonv1 "voice.app/voice/common/v1"
)

func TestInventoryItemKeyDistinguishesFullTuple(t *testing.T) {
	file := "11111111-1111-4111-8111-111111111111"
	owner := "22222222-2222-4222-8222-222222222222"
	sub := "33333333-3333-4333-8333-333333333333"
	scope := "44444444-4444-4444-8444-444444444444"
	keys := []*commonv1.ReferenceInventoryKey{
		{FileId: file, OwnerType: 1, OwnerId: owner},
		{FileId: file, OwnerType: 2, OwnerId: owner},
		{FileId: file, OwnerType: 1, OwnerId: owner, SubresourceId: &sub},
		{FileId: file, OwnerType: 1, OwnerId: owner, ScopeSpaceId: &scope},
	}
	seen := map[string]bool{}
	for _, key := range keys {
		item := inventoryItemKey(&commonv1.ReferenceInventoryEntry{Key: key})
		if seen[item] {
			t.Fatalf("different tuples collide: %s", item)
		}
		seen[item] = true
	}
}
