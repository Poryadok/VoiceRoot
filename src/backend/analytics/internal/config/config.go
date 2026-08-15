package config

import (
	"log"
	"log/slog"
	"os"
	"strings"
)

const DevHashKeyDefault = "compose-dev-analytics-hash-key"

// RequirePersistence is true when ClickHouse and hash key must be configured (staging/prod).
func RequirePersistence() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("ANALYTICS_REQUIRE_PERSISTENCE")))
	return v == "1" || v == "true" || v == "yes"
}

func ResolveHashKey(logger *slog.Logger) string {
	key := strings.TrimSpace(os.Getenv("ANALYTICS_ID_HASH_KEY"))
	if key == "" {
		if RequirePersistence() {
			log.Fatal("ANALYTICS_ID_HASH_KEY required when ANALYTICS_REQUIRE_PERSISTENCE=true")
		}
		if logger != nil {
			logger.Warn("ANALYTICS_ID_HASH_KEY not set; using dev default")
		}
		return DevHashKeyDefault
	}
	if RequirePersistence() && key == DevHashKeyDefault {
		log.Fatal("ANALYTICS_ID_HASH_KEY must not use dev default when ANALYTICS_REQUIRE_PERSISTENCE=true")
	}
	return key
}

func ResolveClickHouseDSN(logger *slog.Logger) string {
	dsn := strings.TrimSpace(os.Getenv("CLICKHOUSE_DSN"))
	if dsn == "" {
		if RequirePersistence() {
			log.Fatal("CLICKHOUSE_DSN required when ANALYTICS_REQUIRE_PERSISTENCE=true")
		}
		if logger != nil {
			logger.Warn("CLICKHOUSE_DSN not set; ingest will buffer but not persist")
		}
	}
	return dsn
}
