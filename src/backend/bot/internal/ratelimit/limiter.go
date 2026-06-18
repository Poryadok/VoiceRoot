package ratelimit

import "google.golang.org/grpc"

// ServerLimiter enforces bot gRPC rate limits (in-memory or Redis-backed).
type ServerLimiter interface {
	UnaryServerInterceptor() grpc.UnaryServerInterceptor
}
