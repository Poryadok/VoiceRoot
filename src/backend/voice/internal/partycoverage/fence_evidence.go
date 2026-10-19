package partycoverage

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// Resource evidence contains actual identities/postconditions, never secret
// credentials. It remains component evidence until the protected composite
// issuer binds complete deployment/admission/history custody and the exact cut.
type PostgresRoleBinding struct {
	DatabaseIdentity [32]byte
	RoleOID          uint32
	RoleName         string
}
type PostgresFenceEvidence struct {
	Protocol       uint32
	Binding        PostgresRoleBinding
	AuthorityBytes []byte
	AuthorityHash  [32]byte
}
type RedisProcessBinding struct{ RunID, Version string }
type RedisFenceEvidence struct {
	Protocol                uint32
	Binding                 RedisProcessBinding
	ACLUser, ACLFilePath    string
	PersistedUserRulesBytes []byte
	PersistedUserRulesHash  [32]byte
	ACLSaveResult           string
}

func ObservePostgresFence(ctx context.Context, admin *pgx.Conn, expected PostgresRoleBinding) (PostgresFenceEvidence, error) {
	if admin == nil || expected.RoleOID == 0 || expected.RoleName == "" || expected.RoleName == admin.Config().User || expected.DatabaseIdentity == ([32]byte{}) {
		return PostgresFenceEvidence{}, ErrUnavailable
	}
	tx, err := admin.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return PostgresFenceEvidence{}, err
	}
	defer tx.Rollback(ctx)
	identity, err := ObserveDatabaseIdentity(ctx, tx)
	if err != nil || identity != expected.DatabaseIdentity {
		return PostgresFenceEvidence{}, ErrUnavailable
	}
	var state struct {
		RoleOID                                                        uint32
		RoleName                                                       string
		Login, Privileged, Prepared, Delegated, Established, Inherited bool
	}
	err = tx.QueryRow(ctx, `SELECT oid,rolname,rolcanlogin,rolsuper OR rolcreaterole OR rolcreatedb OR rolreplication OR rolbypassrls,
 EXISTS(SELECT 1 FROM pg_prepared_xacts WHERE owner=r.rolname),
 EXISTS(SELECT 1 FROM pg_auth_members WHERE roleid=r.oid),
 EXISTS(SELECT 1 FROM pg_stat_activity WHERE usesysid=r.oid),
 EXISTS(SELECT 1 FROM pg_roles a WHERE pg_has_role(r.oid,a.oid,'MEMBER') AND
 (a.rolsuper OR a.rolcreaterole OR a.rolcreatedb OR a.rolreplication OR a.rolbypassrls OR a.rolname IN ('pg_execute_server_program','pg_write_server_files','pg_read_server_files')))
 FROM pg_roles r WHERE oid=$1 AND rolname=$2`, expected.RoleOID, expected.RoleName).Scan(&state.RoleOID, &state.RoleName, &state.Login, &state.Privileged, &state.Prepared, &state.Delegated, &state.Established, &state.Inherited)
	if err != nil || state.Login || state.Privileged || state.Prepared || state.Delegated || state.Established || state.Inherited {
		return PostgresFenceEvidence{}, ErrUnavailable
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return PostgresFenceEvidence{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PostgresFenceEvidence{}, err
	}
	return PostgresFenceEvidence{Protocol: 1, Binding: expected, AuthorityBytes: raw, AuthorityHash: sha256.Sum256(append([]byte("voice.party.postgres-fence-authority.v1\x00"), raw...))}, nil
}

func ObserveRedisProcess(ctx context.Context, admin *redis.Client) (RedisProcessBinding, error) {
	if admin == nil {
		return RedisProcessBinding{}, ErrUnavailable
	}
	info, err := admin.Info(ctx, "server").Result()
	if err != nil {
		return RedisProcessBinding{}, err
	}
	values := map[string]string{}
	for _, line := range strings.Split(info, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok {
			values[key] = value
		}
	}
	b := RedisProcessBinding{values["run_id"], values["redis_version"]}
	if len(b.RunID) != 40 || b.Version == "" {
		return RedisProcessBinding{}, ErrUnavailable
	}
	return b, nil
}

// ObserveRedisFence deliberately re-persists the observed disabled ACL before
// returning its immutable postcondition. A restarted/replaced Redis process is
// unavailable under the old binding until explicit closed-cut reconciliation.
// The enclosing issuer must additionally prove actual persistent-volume and
// execution custody; an ACL path/run ID alone is not that deployment proof.
func ObserveRedisFence(ctx context.Context, admin *redis.Client, expected RedisProcessBinding, user string) (RedisFenceEvidence, error) {
	if admin == nil || user == "" || user == "default" {
		return RedisFenceEvidence{}, ErrUnavailable
	}
	actual, err := ObserveRedisProcess(ctx, admin)
	if err != nil || actual != expected {
		return RedisFenceEvidence{}, ErrUnavailable
	}
	me, err := admin.Do(ctx, "ACL", "WHOAMI").Text()
	if err != nil || me == user {
		return RedisFenceEvidence{}, ErrUnavailable
	}
	for _, command := range [][]interface{}{{"ACL", "SETUSER", user, "on"}, {"CONFIG", "SET", "aclfile", "/not-executed"}, {"MODULE", "LOAD", "/not-executed"}} {
		args := append([]interface{}{"ACL", "DRYRUN", user}, command...)
		result, err := admin.Do(ctx, args...).Text()
		if err != nil || result == "OK" {
			return RedisFenceEvidence{}, ErrUnavailable
		}
	}
	value, err := admin.Do(ctx, "ACL", "GETUSER", user).Result()
	if err != nil {
		return RedisFenceEvidence{}, err
	}
	fields, ok := aclFields(value)
	if !ok {
		return RedisFenceEvidence{}, ErrUnavailable
	}
	flags, ok := fields["flags"].([]interface{})
	if !ok {
		return RedisFenceEvidence{}, ErrUnavailable
	}
	rules := struct {
		Flags                    []string
		Commands, Keys, Channels string
		Selectors                []map[string]string
	}{}
	off := false
	for _, flag := range flags {
		s, ok := flag.(string)
		if !ok {
			return RedisFenceEvidence{}, ErrUnavailable
		}
		off = off || s == "off"
		if s == "on" {
			return RedisFenceEvidence{}, ErrUnavailable
		}
		rules.Flags = append(rules.Flags, s)
	}
	if !off {
		return RedisFenceEvidence{}, ErrUnavailable
	}
	sort.Strings(rules.Flags)
	for key, dest := range map[string]*string{"commands": &rules.Commands, "keys": &rules.Keys, "channels": &rules.Channels} {
		s, ok := fields[key].(string)
		if !ok {
			return RedisFenceEvidence{}, ErrUnavailable
		}
		*dest = s
	}
	selectors, ok := fields["selectors"].([]interface{})
	if !ok {
		return RedisFenceEvidence{}, ErrUnavailable
	}
	for _, selector := range selectors {
		fields, ok := aclFields(selector)
		if !ok || len(fields) != 3 {
			return RedisFenceEvidence{}, ErrUnavailable
		}
		selected := map[string]string{}
		for _, key := range []string{"commands", "keys", "channels"} {
			s, ok := fields[key].(string)
			if !ok {
				return RedisFenceEvidence{}, ErrUnavailable
			}
			selected[key] = s
		}
		rules.Selectors = append(rules.Selectors, selected)
	}
	clients, err := admin.ClientList(ctx).Result()
	if err != nil {
		return RedisFenceEvidence{}, err
	}
	for _, field := range strings.Fields(clients) {
		if field == "user="+user {
			return RedisFenceEvidence{}, ErrUnavailable
		}
	}
	config, err := admin.ConfigGet(ctx, "aclfile").Result()
	if err != nil || config["aclfile"] == "" {
		return RedisFenceEvidence{}, ErrUnavailable
	}
	saved, err := admin.Do(ctx, "ACL", "SAVE").Text()
	if err != nil || saved != "OK" {
		if err == nil {
			err = ErrUnavailable
		}
		return RedisFenceEvidence{}, err
	}
	final, err := ObserveRedisProcess(ctx, admin)
	if err != nil || final != expected {
		return RedisFenceEvidence{}, ErrUnavailable
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return RedisFenceEvidence{}, err
	}
	return RedisFenceEvidence{Protocol: 1, Binding: actual, ACLUser: user, ACLFilePath: config["aclfile"], PersistedUserRulesBytes: raw, PersistedUserRulesHash: sha256.Sum256(append([]byte("voice.party.redis-fence-rules.v1\x00"), raw...)), ACLSaveResult: saved}, nil
}

func aclFields(v interface{}) (map[string]interface{}, bool) {
	out := map[string]interface{}{}
	switch x := v.(type) {
	case map[string]interface{}:
		return x, true
	case map[interface{}]interface{}:
		for k, v := range x {
			key, ok := k.(string)
			if !ok {
				return nil, false
			}
			out[key] = v
		}
	case []interface{}:
		if len(x)%2 != 0 {
			return nil, false
		}
		for i := 0; i < len(x); i += 2 {
			key, ok := x[i].(string)
			if !ok {
				return nil, false
			}
			if _, exists := out[key]; exists {
				return nil, false
			}
			out[key] = x[i+1]
		}
	default:
		return nil, false
	}
	return out, true
}
