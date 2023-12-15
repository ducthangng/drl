package bucket

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Middleware is a middleware function for HTTP handlers to enforce rate limiting
func (limiter *RateLimiterNode) UnaryServerInterceptor(
	ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

	// all the nodes consume the token, even the master
	// this mean that master will do extra work: to refills the tokens.
	if limiter.consumeToken() {
		return handler(ctx, req)
	}

	return nil, status.Error(codes.ResourceExhausted, "Rate limit exceeded")
}

func (limiter *RateLimiterNode) StreamServerInterceptor(
	srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if limiter.consumeToken() {
		return handler(srv, ss)
	}

	return status.Error(codes.ResourceExhausted, "Rate limit exceeded")
}
