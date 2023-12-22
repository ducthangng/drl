package token_bucket

import (
	"context"
	"log"
	"time"

	"github.com/ducthangng/drl/rl/entity"
)

// ConsumeToken checks if there is at least one token in the bucket, and consumes it
func (limiter *RateLimiterNode) Try(ctx context.Context, request entity.Request, val ...interface{}) bool {
	afterDecrease := limiter.RedisClient.DecrBy(context.Background(), "tokens", 1).Val()
	if afterDecrease < 0 {
		// added back the token
		limiter.RedisClient.IncrBy(context.Background(), "tokens", 1)
		return false
	}

	return true
}

func (limiter *RateLimiterNode) SetRateLimit(ctx context.Context, interval entity.Interval) error {
	return limiter.setNewInterval(ctx, int(interval.Interval))
}

func (limiter *RateLimiterNode) GetRateLimit(ctx context.Context, key string) (entity.Interval, error) {
	data, err := limiter.getCurrentInterval(ctx)
	if err != nil {
		return entity.Interval{}, err
	}

	return entity.Interval{
		Interval: data,
	}, nil
}

// RefillTokens periodically adds tokens to the bucket, sometimes the rate gets changed
func (node *RateLimiterNode) refillTokens() {
	log.Println("start refilling tokens at rate: ", node.RefillRate)
	ticker := time.NewTicker(time.Second / time.Duration(node.RefillRate))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			node.addToken()
		case changed := <-node.TickerChanged:
			if changed {
				log.Println("UPDATE TICKER START ", node.RefillRate)

				ticker.Stop()
				node.Mutex.Lock()
				log.Println("UPDATE TICKER MIDDLE ", node.RefillRate)
				ticker = time.NewTicker(time.Second / time.Duration(node.RefillRate))
				node.Mutex.Unlock()

				log.Println("UPDATE TICKER ", node.RefillRate)
			}
		}
	}
}

// addToken adds a token to the bucket
func (node *RateLimiterNode) addToken() {
	if !node.isEstablished {
		return
	}

	if !node.isMaster {
		return
	}

	tokens := node.RedisClient.IncrBy(context.Background(), "tokens", 1).Val()
	log.Println("curr token", tokens)

	if int(tokens) > node.Capacity {
		node.RedisClient.DecrBy(context.Background(), "tokens", 1)
	}

	log.Println("Tokens: ", node.Tokens)
}

func (limiter *RateLimiterNode) Reset(ctx context.Context, key string) error {
	return nil
}
