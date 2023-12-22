package main

import (
	"context"
	"log"

	"github.com/ducthangng/drl/rl/entity"
	"github.com/ducthangng/drl/rl/leaky_bucket"
	"github.com/ducthangng/drl/rl/token_bucket"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LeakyBucketRateLimiter struct {
	bucket *leaky_bucket.RateLimiterNode
}

// Middleware is a middleware function for HTTP handlers to enforce rate limiting
func (t *LeakyBucketRateLimiter) UnaryServerInterceptor(
	ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

	// Generate or retrieve a unique identifier for the request
	requestID := generateRequestID(req) // Implement this function based on your application logic

	// Try to consume a token and process the request
	if t.bucket.Try(ctx, entity.Request{Key: requestID}) == nil {
		// Process the request
		resp, err := handler(ctx, req)

		// After processing the request, remove it from the queue
		removeErr := t.RemoveRequestFromQueue(ctx, requestID)
		if removeErr != nil {
			// Handle the error of removing the request from the queue
			// This can be logging or other error handling logic
			log.Println(removeErr)
		}

		return resp, err
	}

	return nil, status.Error(codes.ResourceExhausted, "Rate limit exceeded")
}

// Implement this method based on how you're tracking requests in the queue
func (node *LeakyBucketRateLimiter) RemoveRequestFromQueue(ctx context.Context, requestID string) error {
	// Logic to remove the request from the queue
	// This can be an LREM command in Redis, for example
	return node.bucket.CleanUp(ctx, entity.Request{
		Key: requestID,
	})
}

// Implement a function to generate or retrieve a unique ID for the request
func generateRequestID(req interface{}) string {
	// Implementation depends on how you can uniquely identify a request
	// This could be a combination of request parameters, a hash, etc.
	return uuid.NewString() // Placeholder, replace with actual logic
}

func NewLeakyBucketRateLimiter(ctx context.Context, redisClient *redis.Client, capacity int, refillRate int) (*TokenBucketRateLimiter, error) {
	b, err := token_bucket.NewDistributedRateLimiter(ctx, redisClient, capacity, refillRate)
	if err != nil {
		return nil, err
	}

	return &TokenBucketRateLimiter{
		bucket: b,
	}, nil
}
