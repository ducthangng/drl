package service

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/ducthangng/drl/pb"
	"github.com/jinzhu/copier"
)

type LaptopStore interface {
	Save(laptop *pb.Laptop) error
	Find(id string) (*pb.Laptop, error)
	Search(ctx context.Context, filter *pb.Filter, found func(laptop *pb.Laptop) error) error
}

type InMemoryLaptopStore struct {
	mutex   sync.RWMutex
	laptops map[string]*pb.Laptop
}

func NewInMemoryLaptopStore() *InMemoryLaptopStore {
	return &InMemoryLaptopStore{
		laptops: make(map[string]*pb.Laptop),
	}
}

func (store *InMemoryLaptopStore) Save(laptop *pb.Laptop) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	other := &pb.Laptop{}
	if err := copier.Copy(other, laptop); err != nil {
		return err
	}

	store.laptops[laptop.Id] = other
	return nil
}

func (store *InMemoryLaptopStore) Find(id string) (*pb.Laptop, error) {

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

	for _, laptop := range store.laptops {

		if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
			log.Print("context is cancelled")
			return errors.New("context is cancelled")
		}

		if isQualified(laptop, filter) {
			time.Sleep(5 * time.Second)

			other := &pb.Laptop{}
			if err := copier.Copy(other, laptop); err != nil {
				return err
			}

			if err := found(other); err != nil {
				return err
			}
		}
	}

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
