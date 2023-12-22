package fix_window_counter

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

func (node *RateLimiterNode) Try(ctx context.Context, request entity.Request, val ...interface{}) (bool, error) {
	node.Mutex.Lock()
	defer node.Mutex.Unlock()

	// Generate a key for the current window
	windowKey := fmt.Sprintf("rate_limit:%s:%d", node.ID, time.Now().Unix()/60) // per-minute window

	// Check the current count for the window
	count, err := node.RedisClient.IncrBy(ctx, windowKey, 1).Result()
	if err != nil && err != redis.Nil {
		return false, err
	}

	if count >= int64(node.Capacity) {
		// Capacity exceeded, deny the request
		if err = node.RedisClient.Decr(ctx, windowKey).Err(); err != nil {
			return false, err
		}

		return false, nil
	}

	return true, nil
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

func (node *RateLimiterNode) CleanUp(ctx context.Context, request entity.Request) error {
	return nil
}
