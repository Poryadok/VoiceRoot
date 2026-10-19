package main

import (
	"crypto/tls"
	"github.com/jackc/pgx/v5/pgxpool"
	"reflect"
	"testing"
)

func TestFixtureDSNCredentialRoundTrip(t *testing.T) {
	for name, source := range map[string]string{
		"uri":                   "postgres://original:old@alpha.example:5432/db%20name?sslmode=require&connect_timeout=7&application_name=fixture%20%27%5C&options=-c%20search_path%3Dpublic&pool_max_conns=7&default_query_exec_mode=simple_protocol",
		"uri_query_credentials": "postgresql://old:old@alpha.example/db?user=original&password=original&sslmode=prefer&host=alpha.example,beta.example&port=5432,6432&connect_timeout=4&target_session_attrs=read-write",
		"keyword":               `host='alpha.example,beta.example' port='5432,6432' dbname='db name' user='original user' password='original' sslmode=prefer connect_timeout=9 application_name='fixture \'quoted\' \\ name' pool_max_conns=8 pool_min_conns=1`,
	} {
		t.Run(name, func(t *testing.T) {
			before, err := pgxpool.ParseConfig(source)
			if err != nil {
				t.Fatal("invalid test source settings")
			}
			login, password := "fixture ' \\ @:/?&=% λ", "secret ' \\ @:/?&=% λ"
			exported, err := fixtureDSNWithCredentials(source, login, password)
			if err != nil {
				t.Fatal("credential serialization failed")
			}
			after, err := pgxpool.ParseConfig(exported)
			if err != nil {
				t.Fatal("serialized connection did not parse")
			}
			if after.ConnConfig.User != login || after.ConnConfig.Password != password {
				t.Fatal("credential roundtrip changed identity")
			}
			b, a := before.ConnConfig, after.ConnConfig
			if b.Host != a.Host || b.Port != a.Port || b.Database != a.Database || b.ConnectTimeout != a.ConnectTimeout || !reflect.DeepEqual(b.RuntimeParams, a.RuntimeParams) || b.DefaultQueryExecMode != a.DefaultQueryExecMode || b.StatementCacheCapacity != a.StatementCacheCapacity || b.DescriptionCacheCapacity != a.DescriptionCacheCapacity || before.MaxConns != after.MaxConns || before.MinConns != after.MinConns || !sameFixtureTLS(b.TLSConfig, a.TLSConfig) || len(b.Fallbacks) != len(a.Fallbacks) {
				t.Fatal("noncredential connection settings changed")
			}
			for i, expected := range b.Fallbacks {
				actual := a.Fallbacks[i]
				if expected.Host != actual.Host || expected.Port != actual.Port || !sameFixtureTLS(expected.TLSConfig, actual.TLSConfig) {
					t.Fatal("TLS or fallback host settings changed")
				}
			}
		})
	}
}

func sameFixtureTLS(a, b *tls.Config) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.ServerName == b.ServerName && a.MinVersion == b.MinVersion && a.MaxVersion == b.MaxVersion && a.InsecureSkipVerify == b.InsecureSkipVerify && ((a.RootCAs == nil && b.RootCAs == nil) || (a.RootCAs != nil && b.RootCAs != nil && a.RootCAs.Equal(b.RootCAs)))
}
