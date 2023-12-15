package bucket

import (
	"context"
	"log"
	"time"
)

// ConsumeToken checks if there is at least one token in the bucket, and consumes it
func (limiter *RateLimiterNode) consumeToken() bool {
	limiter.Mutex.Lock()
	defer limiter.Mutex.Unlock()

	tokens, err := limiter.RedisClient.Get(context.Background(), "tokens").Int()
	if err != nil {
		log.Println(err)
	}

	log.Println("Consuming token", tokens, limiter.Tokens)
	if tokens > 0 {
		tokens--
		limiter.Tokens = tokens

		// Set the new value in Redis
		err = limiter.RedisClient.Set(context.Background(), "tokens", tokens, 0).Err()
		if err != nil {
			log.Println(err)
		}

		return true
	}

	return false
}

// RefillTokens periodically adds tokens to the bucket
func (node *RateLimiterNode) refillTokens() {
	ticker := time.NewTicker(time.Second / time.Duration(node.RefillRate))
	defer ticker.Stop()

	for range ticker.C {
		node.addToken()
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

	tokens, err := node.RedisClient.Get(context.Background(), "tokens").Int()
	if err != nil {
		log.Println(err)
	}

	log.Println("Adding token", tokens)

	tokens++
	if tokens > node.Capacity {
		node.Tokens = node.Capacity
		tokens = node.Capacity
	} else {
		node.Tokens = tokens
	}

	// Set the new value in Redis
	err = node.RedisClient.Set(context.Background(), "tokens", tokens, 0).Err()
	if err != nil {
		log.Println(err)
	}

	log.Println("Tokens: ", node.Tokens)
}
