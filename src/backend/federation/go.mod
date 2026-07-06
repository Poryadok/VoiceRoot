module voice/backend/federation

go 1.26

require voice/backend/pkg v0.0.0

require (
	golang.org/x/sys v0.32.0 // indirect
	google.golang.org/grpc v1.70.0 // indirect
)

replace voice/backend/pkg => ../pkg
