package service

import (
	"context"
	"fmt"
	"log"

	"github.com/ducthangng/drl/pb"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LaptopServer struct {
	Store LaptopStore
	pb.UnimplementedLaptopServiceServer
}

func NewLaptopServer(store LaptopStore) *LaptopServer {
	return &LaptopServer{
		Store: store,
	}
}

func (server *LaptopServer) CreateLaptop(ctx context.Context, req *pb.CreateLaptopRequest) (*pb.CreateLaptopResponse, error) {
	laptop := req.GetLaptop()
	log.Printf("receive a create-laptop request with id: %s", laptop.Id)

	// check if id is valid
	if len(laptop.Id) > 0 {
		_, err := uuid.Parse(laptop.Id)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "laptop id is not a valid uuid: %v", err)
		}
	} else {
		laptop.Id = uuid.NewString()
	}

	// check if the store already have that laptop
	// if found, return already-exists error
	// if not found, save the laptop to the store
	dbLaptop, err := server.Store.Find(laptop.Id)
	if err != nil {
		return nil, fmt.Errorf("cannot find laptop: %w", err)
	}

	if dbLaptop != nil {
		return nil, status.Errorf(codes.AlreadyExists, "laptop with id already exists: %s", laptop.Id)
	}

	if err := server.Store.Save(laptop); err != nil {
		return nil, fmt.Errorf("cannot save laptop to the store: %w", err)
	}

	return &pb.CreateLaptopResponse{
		Id: laptop.Id,
	}, nil
}

func (server *LaptopServer) SearchLaptop(req *pb.SearchLaptopRequest, stream pb.LaptopService_SearchLaptopServer) error {
	filter := req.GetFilter()
	log.Printf("receive a search-laptop request with filter: %v", filter)

	err := server.Store.Search(stream.Context(), filter, func(laptop *pb.Laptop) error {
		res := &pb.SearchLaptopResponse{Laptop: laptop}
		err := stream.Send(res)
		if err != nil {
			return err
		}

		log.Printf("sent laptop with id: %s", laptop.GetId())

		return nil
	})

	if err != nil {
		return fmt.Errorf("cannot search laptop: %w", err)
	}

	return nil
}
