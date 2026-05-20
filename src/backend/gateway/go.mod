module voice/backend/gateway

go 1.26

require (
	github.com/gorilla/websocket v1.5.3
	github.com/stretchr/testify v1.10.0
	google.golang.org/grpc v1.70.0
	google.golang.org/protobuf v1.36.5
	voice.app/voice/chat v0.0.0
	voice.app/voice/common v0.0.0
	voice.app/voice/messaging v0.0.0
	voice.app/voice/social v0.0.0
	voice.app/voice/user v0.0.0
	voice/backend/pkg v0.0.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/net v0.32.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241202173237-19429a94021a // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace voice/backend/pkg => ../pkg

replace voice.app/voice/user => ../user/pb/voice/user

replace voice.app/voice/social => ../user/pb/voice/social

replace voice.app/voice/common => ../user/pb/voice/common

replace voice.app/voice/chat => ../chat/pb/voice/chat

replace voice.app/voice/messaging => ../messaging/pb/voice/messaging
