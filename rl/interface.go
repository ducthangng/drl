package rate_limiter

import (
	"context"

	"github.com/ducthangng/drl/rl/entity"
)

type DistirbutedRateLimiter interface {
	// DRL represent a distributed rate limiter interface
	// consists of core functions that each limiter should inplement:

	// Try is when a client wants to check if it can process that request.
	// According to each limiter's algorithm, it will return true or false
	Try(ctx context.Context, request entity.Request, val ...interface{}) (bool, error)

	// SetRateLimit sets or updates the rate limit for a given key.
	SetRateLimit(ctx context.Context, interval entity.Interval) error

	// GetRateLimit retrieves the current rate limit settings for a given key.
	GetRateLimit(ctx context.Context, key string) (entity.Interval, error)

	// Reset clears the rate limiting data for a specific key.
	Reset(ctx context.Context, key string) error

	CleanUp(ctx context.Context, request entity.Request) error
}
