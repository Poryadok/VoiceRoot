module voice/backend/realtime

go 1.26

require (
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	google.golang.org/grpc v1.70.0
	voice.app/voice/chat v0.0.0
	voice.app/voice/common v0.0.0
	voice/backend/pkg v0.0.0
)

require (
	golang.org/x/net v0.32.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241202173237-19429a94021a // indirect
	google.golang.org/protobuf v1.36.5 // indirect
)

replace voice/backend/pkg => ../pkg

replace voice.app/voice/chat => ../chat/pb/voice/chat

replace voice.app/voice/common => ../user/pb/voice/common
