package principalruntime

import (
	"context"
	"errors"
	"voice/backend/pkg/authoritysource"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
)

type AuthoritySourceReader interface {
	authoritysource.Reader
	CheckAuthoritySourceSchema(context.Context) error
}

// ActivateAuthoritySource must run before server construction. Runtime and its
// serving set are immutable after construction; reads repeat the catalog check
// so a later maintenance/schema change cannot leave authority available.
func (r *Runtime) ActivateAuthoritySource(ctx context.Context, reader AuthoritySourceReader) error {
	if r == nil || !r.authoritySourceEnabled || r.resolver == nil || r.replay == nil || reader == nil || r.authoritySourceServer != nil {
		return errors.New("authority source activation dependencies incomplete")
	}
	if err := reader.CheckAuthoritySourceSchema(ctx); err != nil {
		return errors.New("authority source schema preflight failed")
	}
	server, err := authoritysource.NewServer(authorityv1.AuthorityOwner_AUTHORITY_OWNER_ROLE, reader)
	if err != nil {
		return err
	}
	interceptor, err := authoritysource.UnaryInterceptor(authorityv1.AuthorityOwner_AUTHORITY_OWNER_ROLE, r.resolver.Resolve, r.recordReplay)
	if err != nil {
		return err
	}
	r.authoritySourceServer, r.authoritySourceInterceptor = server, interceptor
	return nil
}
