package service

import (
	"context"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/ducthangng/drl/pb"
	"github.com/go-redis/redis/v8"
	"github.com/jinzhu/copier"
)

type LaptopStore interface {
	Save(ctx context.Context, laptop *pb.Laptop) error
	Find(ctx context.Context, id string) (*pb.Laptop, error)
	Search(ctx context.Context, filter *pb.Filter, found func(laptop *pb.Laptop) error) error
}

type InMemoryLaptopStore struct {
	mutex       sync.RWMutex
	redisClient *redis.Client
	laptops     map[string]*pb.Laptop
}

func NewInMemoryLaptopStore(redisClient *redis.Client) *InMemoryLaptopStore {
	return &InMemoryLaptopStore{
		laptops:     make(map[string]*pb.Laptop),
		redisClient: redisClient,
	}
}

func (store *InMemoryLaptopStore) Save(ctx context.Context, laptop *pb.Laptop) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	other := &pb.Laptop{}
	if err := copier.Copy(other, laptop); err != nil {
		return err
	}

	// store.laptops[laptop.Id] = other
	// store to redis
	data, err := sonic.MarshalString(laptop)
	if err != nil {
		return err
	}

	if err = store.redisClient.Set(ctx, "laptop:"+laptop.Id, data, 0).Err(); err != nil {
		return err
	}

	return nil
}

func (store *InMemoryLaptopStore) Find(ctx context.Context, id string) (*pb.Laptop, error) {

	store.mutex.RLock()
	defer store.mutex.RUnlock()

	laptop, found := store.laptops[id]
	if !found {
		return nil, nil
	}

	other := &pb.Laptop{}
	if err := copier.Copy(other, laptop); err != nil {
		return nil, err
	}

	return other, nil
}

func (store *InMemoryLaptopStore) Search(ctx context.Context, filter *pb.Filter, found func(laptop *pb.Laptop) error) error {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	var allKeys []string
	var cursor uint64
	var err error

	// have a go routine to check if context is time out
	// errChan := make(chan error, 1)
	// done := make(chan bool, 1)

	// go func() {
	// 	<-ctx.Done()
	// 	if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
	// 		log.Print("context is cancelled")
	// 		errChan <- ctx.Err()
	// 	}
	// }()

	// Iterate through all keys in the Redis database
	for {
		var keys []string
		keys, cursor, err = store.redisClient.Scan(ctx, cursor, "*", 0).Result()
		if err != nil {
			return err
		}

		allKeys = append(allKeys, keys...)

		// If the cursor returned is 0, it means we have iterated through all the keys
		if cursor == 0 {
			break
		}
	}

	// retrive all keys successfully
	for _, key := range allKeys {
		var laptop *pb.Laptop
		var laptopStr string
		laptopStr, err = store.redisClient.Get(ctx, key).Result()
		if err != nil {
			return err
		}

		if err = sonic.UnmarshalString(laptopStr, laptop); err != nil {
			return err
		}

		if isQualified(laptop, filter) {
			other := &pb.Laptop{}
			if err := copier.Copy(other, laptop); err != nil {
				return err
			}

			if err := found(other); err != nil {
				return err
			}
		}
	}

	// close(done)

	// select {
	// case err := <-errChan:
	// 	return err
	// case <-done:
	// 	return nil
	// }

	return nil
}

func isQualified(laptop *pb.Laptop, filter *pb.Filter) bool {
	if laptop.GetPriceUsd() > filter.GetMaxPriceUsd() {
		return false
	}

	if laptop.GetCpu().GetNumberCores() < filter.GetMinCpuCore() {
		return false
	}

	if laptop.GetCpu().GetMinGhz() < float64(filter.GetMinCpuGhz()) {
		return false
	}

	if toBit(laptop.GetRam()) < toBit(filter.GetMinRam()) {
		return false
	}

	return true
}

func toBit(memory *pb.Memory) uint64 {
	value := memory.GetValue()
	switch memory.GetUnit() {
	case pb.Memory_BIT:
		return value
	case pb.Memory_BYTE:
		return value << 3
	case pb.Memory_KILOBYTE:
		return value << 13
	case pb.Memory_MEGABYTE:
		return value << 23
	case pb.Memory_GIGABYTE:
		return value << 33
	case pb.Memory_TERABYTE:
		return value << 43
	default:
		return 0
	}
}
