package nativeinstaller

import (
	"context"
	"errors"
	"reflect"

	"github.com/jackc/pgx/v5"
)

type legacy24FunctionAccess struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Access    string `json:"access"`
}

func captureLegacy24FunctionsTx(ctx context.Context, tx pgx.Tx, original []legacy24FunctionAccess) ([]legacy24FunctionAccess, error) {
	query := `SELECT p.proname,pg_get_function_identity_arguments(p.oid),jsonb_build_array(pg_get_userbyid(p.proowner),p.proacl)::text FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND NOT EXISTS(SELECT 1 FROM pg_depend d WHERE d.classid='pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e') ORDER BY p.proname,pg_get_function_identity_arguments(p.oid)`
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return nil, errors.New("HOLD: original function rights unavailable")
	}
	all := make([]legacy24FunctionAccess, 0)
	for rows.Next() {
		var f legacy24FunctionAccess
		if rows.Scan(&f.Name, &f.Arguments, &f.Access) != nil || len(all) >= 1000 {
			rows.Close()
			return nil, errors.New("HOLD: original function rights unavailable")
		}
		all = append(all, f)
	}
	if rows.Err() != nil {
		rows.Close()
		return nil, errors.New("HOLD: original function rights unavailable")
	}
	rows.Close()
	if original == nil {
		return all, nil
	}
	matched := make([]legacy24FunctionAccess, 0, len(original))
	j := 0
	for _, f := range all {
		if j < len(original) && f.Name == original[j].Name && f.Arguments == original[j].Arguments {
			matched = append(matched, f)
			j++
		}
	}
	if !reflect.DeepEqual(matched, original) {
		return nil, errors.New("HOLD: original function rights changed")
	}
	return matched, nil
}
