package authoritysource

import (
	"github.com/google/uuid"
	"testing"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
)

func TestQualifiedScopeExplicitRoleSpaceOnly(t *testing.T) {
	for _, owner := range []authorityv1.AuthorityOwner{authorityv1.AuthorityOwner_AUTHORITY_OWNER_ROLE, authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE, authorityv1.AuthorityOwner_AUTHORITY_OWNER_AUTH, authorityv1.AuthorityOwner_AUTHORITY_OWNER_USER, authorityv1.AuthorityOwner_AUTHORITY_OWNER_GAME_INTEGRATION} {
		scope := &authorityv1.SourceScope{SchemaVersion: 2, SpaceId: uuid.NewString()}
		if owner == authorityv1.AuthorityOwner_AUTHORITY_OWNER_GAME_INTEGRATION {
			scope.EnvironmentId = uuid.NewString()
		}
		err := ValidateScope(owner, scope)
		allowed := owner == authorityv1.AuthorityOwner_AUTHORITY_OWNER_ROLE || owner == authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE
		if (err == nil) != allowed {
			t.Fatalf("owner %v qualified scope accepted=%v", owner, err == nil)
		}
	}
}
