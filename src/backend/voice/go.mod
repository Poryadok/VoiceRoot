module voice/backend/voice

go 1.26

require (
	github.com/google/uuid v1.6.0
	github.com/nats-io/nats.go v1.39.1
	github.com/prometheus/client_golang v1.20.5
	github.com/redis/go-redis/v9 v9.7.0
	github.com/stretchr/testify v1.10.0
	google.golang.org/grpc v1.70.0
	google.golang.org/protobuf v1.36.5
	voice.app/voice/calls v0.0.0
	voice.app/voice/chat v0.0.0
	voice.app/voice/common v0.0.0
	voice.app/voice/events v0.0.0
	voice.app/voice/social v0.0.0-00010101000000-000000000000
	voice.app/voice/space v0.0.0
	voice.app/voice/user v0.0.0
	voice/backend/pkg v0.0.0-00010101000000-000000000000
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nats-io/nkeys v0.4.9 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/net v0.32.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241202173237-19429a94021a // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace voice/backend/pkg => ../pkg

replace voice.app/voice/calls => ./pb/voice/calls

replace voice.app/voice/chat => ../chat/pb/voice/chat

replace voice.app/voice/common => ../user/pb/voice/common

replace voice.app/voice/events => ../messaging/pb/voice/events

replace voice.app/voice/space => ./pb/voice/space

replace voice.app/voice/messaging => ../messaging/pb/voice/messaging

replace voice.app/voice/user => ../user/pb/voice/user

replace voice.app/voice/social => ../user/pb/voice/social

replace voice.app/voice/role => ../role/pb/voice/role

replace voice.app/voice/file => ../file/pb/voice/file
