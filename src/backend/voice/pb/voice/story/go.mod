module voice.app/voice/story

go 1.26

require (
	google.golang.org/grpc v1.70.0
	google.golang.org/protobuf v1.36.5
	voice.app/voice/common v0.0.0
)

replace voice.app/voice/common => ../../../../user/pb/voice/common
