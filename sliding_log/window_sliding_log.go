package sliding_log

import (
	"context"
	"fmt"
	"time"

	"github.com/ducthangng/drl/singleton"
	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RateLimiter struct {
	client  *redis.Client
	limit   int
	window  time.Duration
	context context.Context
}

var Limiter *RateLimiter

func NewRateLimiter(client *redis.Client, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		client:  client,
		limit:   limit,
		window:  window,
		context: context.Background(),
	}
}

func (rl *RateLimiter) IsAllowed(key string) (bool, error) {
	ctx := rl.context
	now := time.Now()

	// Use a Redis pipeline to execute commands atomically
	_, err := rl.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		// Remove outdated timestamps
		pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprint(now.Add(-rl.window).UnixNano()))

		// Add the timestamp of the new request
		pipe.ZAdd(ctx, key, &redis.Z{
			Score:  float64(now.UnixNano()),
			Member: now.String(),
		})

		// Count the number of requests in the current window
		pipe.ZCount(ctx, key, fmt.Sprint(now.Add(-rl.window).UnixNano()), fmt.Sprint(now.UnixNano()))

		return nil
	})

	if err != nil {
		return false, err
	}

	// Get the count of requests in the window
	count, err := rl.client.ZCount(ctx, key, fmt.Sprint(now.Add(-rl.window).UnixNano()), fmt.Sprint(now.UnixNano())).Result()
	if err != nil {
		return false, err
	}

	// Check if the count is within the limit
	return count <= int64(rl.limit), nil
}

func (limiter *RateLimiter) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Here we use the full method name as the key, but you can use a different logic
		allowed, err := limiter.IsAllowed(info.FullMethod)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "rate limiter error: %v", err)
		}
		if !allowed {
			return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
		}
		// Call the handler to proceed with the actual service method
		return handler(ctx, req)
	}
}
func SetUp() {
	// Create DistributedRateLimiter
	limiter := NewRateLimiter(singleton.GetRedisClient(), 5, 1*time.Second)
	Limiter = limiter

	// Start token refill in the background
}

func GetLimiter() *RateLimiter {
	if Limiter == nil {
		SetUp()
	}

	return Limiter
}
