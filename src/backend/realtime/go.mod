module voice/backend/realtime

go 1.26

require (
	github.com/gorilla/websocket v1.5.3
	voice/backend/pkg v0.0.0
)

replace voice/backend/pkg => ../pkg
