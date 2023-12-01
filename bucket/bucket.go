package bucket

import (
	"context"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var Limiter *DistributedRateLimiter

// DistributedRateLimiter represents a distributed rate limiter using Token Bucket algorithm
type DistributedRateLimiter struct {
	Mutext      sync.Mutex
	RedisClient *redis.Client
	Capacity    int
	Tokens      int
	RefillRate  int
}

// NewDistributedRateLimiter creates a new instance of DistributedRateLimiter
func NewDistributedRateLimiter(redisClient *redis.Client, capacity, refillRate int) *DistributedRateLimiter {
	return &DistributedRateLimiter{
		RedisClient: redisClient,
		Capacity:    capacity,
		Tokens:      capacity,
		RefillRate:  refillRate,
	}
}

// RefillTokens periodically adds tokens to the bucket
func (limiter *DistributedRateLimiter) RefillTokens() {
	ticker := time.NewTicker(time.Second / time.Duration(limiter.RefillRate))
	defer ticker.Stop()

	for range ticker.C {
		limiter.addToken()
	}
}

// addToken adds a token to the bucket
func (limiter *DistributedRateLimiter) addToken() {
	limiter.Mutext.Lock()
	defer limiter.Mutext.Unlock()

	limiter.Tokens++
	if limiter.Tokens > limiter.Capacity {
		limiter.Tokens = limiter.Capacity
	}
}

// ConsumeToken checks if there is at least one token in the bucket, and consumes it
func (limiter *DistributedRateLimiter) ConsumeToken() bool {
	limiter.Mutext.Lock()
	defer limiter.Mutext.Unlock()

	if limiter.Tokens > 0 {
		limiter.Tokens--
		return true
	}

	return false
}

// Middleware is a middleware function for HTTP handlers to enforce rate limiting
func (limiter *DistributedRateLimiter) UnaryServerInterceptor(
	ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

	if limiter.ConsumeToken() {
		return handler(ctx, req)
	}
	return nil, status.Error(codes.ResourceExhausted, "Rate limit exceeded")
}

func (limiter *DistributedRateLimiter) StreamServerInterceptor(
	srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if limiter.ConsumeToken() {
		return handler(srv, ss)
	}
	return status.Error(codes.ResourceExhausted, "Rate limit exceeded")
}

func SetUp() {
	// Set up Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})

	// Create DistributedRateLimiter
	limiter := NewDistributedRateLimiter(redisClient, 10, 1)

	Limiter = limiter

	// Start token refill in the background
	go limiter.RefillTokens()

	return
}

func GetLimiter() *DistributedRateLimiter {
	if Limiter == nil {
		SetUp()
	}

	return Limiter
}
