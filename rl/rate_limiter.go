package rate_limiter

import "context"

type DistirbutedRateLimiter interface {
	// DRL represent a distributed rate limiter interface
	// consists of core functions that each limiter should inplement:

	// Try is when a client wants to check if it can process that request.
	// According to each limiter's algorithm, it will return true or false
	Try(ctx context.Context, request Request, val ...interface{}) (bool, error)

	// SetRateLimit sets or updates the rate limit for a given key.
	SetRateLimit(ctx context.Context, key string, limit int, burst int) error

	// GetRateLimit retrieves the current rate limit settings for a given key.
	GetRateLimit(ctx context.Context, key string) (Request, error)

	// Reset clears the rate limiting data for a specific key.
	Reset(ctx context.Context, key string) error
}

type Request struct {
	// Request is the struct that will be passed to the limiter to check if it can process the request
	// As each limiter has its own algorithm, the appropriate fields should be filled on a per-limiter basis
	// For example, the Leaky Bucket limiter will only need the key and the limit, while the Token Bucket limiter will need the key, the limit, and the burst
	Key                 string
	Limit               int
	Burst               int
	OriginatedTimeStamp int64
}
