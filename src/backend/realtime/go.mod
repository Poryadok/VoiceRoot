module voice/backend/realtime

go 1.26

require (
	github.com/alicebob/miniredis/v2 v2.34.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/nats-io/nats-server/v2 v2.10.24
	github.com/nats-io/nats.go v1.39.1
	github.com/redis/go-redis/v9 v9.7.0
	google.golang.org/grpc v1.70.0
	google.golang.org/protobuf v1.36.5
	voice.app/voice/chat v0.0.0
	voice.app/voice/common v0.0.0
	voice.app/voice/events v0.0.0
	voice.app/voice/user v0.0.0
	voice/backend/pkg v0.0.0
)

require (
	github.com/alicebob/gopher-json v0.0.0-20230218143504-906a9b012302 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/minio/highwayhash v1.0.3 // indirect
	github.com/nats-io/jwt/v2 v2.7.3 // indirect
	github.com/nats-io/nkeys v0.4.9 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/net v0.32.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/time v0.8.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241202173237-19429a94021a // indirect
)

replace voice/backend/pkg => ../pkg

replace voice.app/voice/chat => ../chat/pb/voice/chat

replace voice.app/voice/common => ../user/pb/voice/common

replace voice.app/voice/events => ../messaging/pb/voice/events

replace voice.app/voice/user => ../user/pb/voice/user
