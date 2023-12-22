package leaky_bucket

import (
	"context"
	"fmt"
	"math/rand"

	"time"

	"github.com/ducthangng/drl/rl/entity"
)

/*

Atomicity and Concurrency: Ensure that your queue operations are atomic and handle concurrency correctly.
	This is especially important in a distributed environment where multiple instances might be interacting with the queue simultaneously.

Performance: For very high-throughput systems, continually updating a Redis data structure might become a bottleneck.
	Monitor performance and consider batching operations if needed.

Error Handling: Implement robust error handling, especially for network-related issues with Redis.

Testing: Thoroughly test the queue mechanism under different loads to ensure it behaves as expected.

*/

const (
	ProcessingQueue = "processingQueue"
	LockKey         = "queueLock"
	LockTimeout     = 5 * time.Second
)

func (node *RateLimiterNode) periodicalCleanUp(ctx context.Context, interval time.Duration) {
	// get all the requests in the queue
	// check if the timestamp of the request is older than the interval
	// if yes, remove the request from the queue
	// if no, stop the process

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		requests, err := node.RedisClient.LRange(ctx, ProcessingQueue, 0, -1).Result()
		if err != nil {
			panic(err)
		}

		for _, request := range requests {
			requestWithTimestamp := request
			requestTimestamp := requestWithTimestamp[len(requestWithTimestamp)-10:]
			timestamp, err := time.Parse(time.UnixDate, requestTimestamp)
			if err != nil {
				panic(err)
			}

			if time.Now().After(timestamp.Add(interval)) {
				if err := node.removeRequestFromQueue(request); err != nil {
					panic(err)
				}
			}
		}
	}
}

func (node *RateLimiterNode) Try(ctx context.Context, request entity.Request, val ...interface{}) error {
	var lockAcquired bool

	// check if the queue is full
	if node.RedisClient.LLen(ctx, ProcessingQueue).Val() >= int64(node.Capacity) {
		return nil
	}

	for i := 0; i < node.MaxRetries; i++ {
		lockAcquired, err := node.RedisClient.SetNX(ctx, LockKey, "1", LockTimeout).Result()
		if err != nil {
			return err
		}
		if lockAcquired {
			defer node.RedisClient.Del(ctx, LockKey).Err() // Release the lock
			break
		}
		time.Sleep(time.Duration(node.RetryIntervalInSecond*(1<<i)+rand.Intn(100)) * time.Millisecond)
	}

	if !lockAcquired {
		return fmt.Errorf("unable to acquire lock after retries")
	}

	timestamp := time.Now().Unix() // Current Unix timestamp
	requestWithTimestamp := fmt.Sprintf("%s:%d", request.Key, timestamp)
	if err := node.addRequestToQueue(requestWithTimestamp); err != nil {
		return err
	}

	return nil
}

func (node *RateLimiterNode) CleanUp(ctx context.Context, request entity.Request) error {
	return node.removeRequestFromQueue(request.Key)
}

func (node *RateLimiterNode) addRequestToQueue(requestIDWithTimestamp string) error {
	ctx := context.Background()
	return node.RedisClient.LPush(ctx, ProcessingQueue, requestIDWithTimestamp).Err()
}

func (node *RateLimiterNode) removeRequestFromQueue(requestID string) error {
	ctx := context.Background()
	return node.RedisClient.LRem(ctx, ProcessingQueue, 1, requestID).Err()
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

func (node *RateLimiterNode) refillBucket() {
	ticker := time.NewTicker(time.Second / time.Duration(node.RefillRate))
	defer ticker.Stop()

	for range ticker.C {
		node.Mutex.Lock()
		if node.Tokens < node.Capacity {
			node.Tokens++
		}
		node.Mutex.Unlock()
	}
}
