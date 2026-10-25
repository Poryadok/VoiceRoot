module voice/backend/mmadmissionctl

go 1.26.5

require (
	github.com/jackc/pgx/v5 v5.7.6
	github.com/mattn/go-sqlite3 v1.14.32
	voice/backend/pkg v0.0.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/stretchr/testify v1.10.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/text v0.24.0 // indirect
)

replace voice/backend/pkg => ../pkg
