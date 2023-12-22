package sliding_window_counter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ducthangng/drl/rl/entity"
	"github.com/go-redis/redis/v8"
	"github.com/go-zookeeper/zk"
)

type RateLimiterNode struct {
	ID                    string
	Mutex                 sync.Mutex
	RedisClient           *redis.Client
	Capacity              int
	Tokens                int
	RefillRate            int // Number of tokens to add per second
	Conn                  *zk.Conn
	NodeChan              <-chan zk.Event
	IntervalNodeChan      <-chan zk.Event
	TickerChanged         chan bool
	MaxRetries            int
	RetryIntervalInSecond int

	otherNodeIDs  []string // List of node IDs with their addresses\
	isEstablished bool
	isMaster      bool
}

// NewDistributedRateLimiter creates a new instance of DistributedRateLimiter
func NewDistributedRateLimiter(ctx context.Context, redisClient *redis.Client, capacity, refillRate int) (*RateLimiterNode, error) {
	rateLimiter := &RateLimiterNode{
		RedisClient:   redisClient,
		Capacity:      capacity,
		Tokens:        capacity,
		RefillRate:    refillRate,
		TickerChanged: make(chan bool, 1),
	}

	return rateLimiter, nil
}

func (node *RateLimiterNode) Try(ctx context.Context, requestID string) error {
	now := time.Now().UnixNano()            // Current time in nanoseconds
	windowStart := now - int64(time.Second) // 1 second window

	key := fmt.Sprintf("rate_limit:%s", node.ID)

	// Remove old requests outside of the current window
	_, err := node.RedisClient.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart)).Result()
	if err != nil {
		return err
	}

	// Check the number of requests in the current window
	count, err := node.RedisClient.ZCount(ctx, key, fmt.Sprintf("%d", windowStart), fmt.Sprintf("%d", now)).Result()
	if err != nil {
		return err
	}

	if count >= int64(node.Capacity) {
		// Capacity exceeded, deny the request
		return nil
	}

	// Add the new request timestamp to the window
	_, err = node.RedisClient.ZAdd(ctx, key, &redis.Z{
		Score:  float64(now),
		Member: requestID,
	}).Result()
	if err != nil {
		return err
	}

	return nil
}

func (node *RateLimiterNode) SetRateLimit(ctx context.Context, interval entity.Interval) error {
	return nil
}

func (node *RateLimiterNode) GetRateLimit(ctx context.Context, key string) (entity.Interval, error) {
	return entity.Interval{}, nil
}

func (node *RateLimiterNode) Reset(ctx context.Context, key string) error {
	return nil
}

// func (node *RateLimiterNode) Try(ctx context.Context, requestID string) (bool, error) {
// 	now := time.Now().UnixNano()            // Current time in nanoseconds
// 	windowStart := now - int64(time.Second) // 1 second window
// 	key := fmt.Sprintf("rate_limit:%s", node.ID)

// 	luaScript := `
//         local key = KEYS[1]
//         local windowStart = tonumber(ARGV[1])
//         local now = tonumber(ARGV[2])
//         local capacity = tonumber(ARGV[3])
//         local requestID = ARGV[4]

//         -- Remove old requests outside of the current window
//         redis.call('ZREMRANGEBYSCORE', key, 0, windowStart)

//         -- Check the number of requests in the current window
//         local count = redis.call('ZCOUNT', key, windowStart, now)

//         if count < capacity then
//             -- Add the new request timestamp to the window
//             redis.call('ZADD', key, now, requestID)
//             return true
//         else
//             return false
//         end
//     `

// 	script := redis.NewScript(luaScript)
// 	result, err := script.Run(ctx, node.RedisClient, []string{key}, windowStart, now, node.Capacity, requestID).Result()
// 	if err != nil {
// 		return false, err
// 	}

// 	// The result from Lua is returned as int64, where 1 means allowed, 0 means not allowed
// 	return result.(int64) == 1, nil
// }
