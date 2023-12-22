package main

import (
	"context"

	"github.com/ducthangng/drl/rl/entity"
	"github.com/ducthangng/drl/rl/token_bucket"
	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TokenBucketRateLimiter struct {
	bucket *token_bucket.RateLimiterNode
}

// Middleware is a middleware function for HTTP handlers to enforce rate limiting
func (t *TokenBucketRateLimiter) UnaryServerInterceptor(
	ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

	// all the nodes consume the token, even the master
	// this mean that master will do extra work: to refills the tokens.
	if t.bucket.Try(ctx, entity.Request{}) {
		return handler(ctx, req)
	}

	return nil, status.Error(codes.ResourceExhausted, "Rate limit exceeded")
}

func NewTokenBucketRateLimiter(ctx context.Context, redisClient *redis.Client, capacity int, refillRate int) (*TokenBucketRateLimiter, error) {
	b, err := token_bucket.NewDistributedRateLimiter(ctx, redisClient, capacity, refillRate)
	if err != nil {
		return nil, err
	}

	return &TokenBucketRateLimiter{
		bucket: b,
	}, nil
}
