package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/ducthangng/drl/pb"
	"github.com/ducthangng/drl/service"
	"github.com/ducthangng/drl/singleton"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
)

func main() {
	redisClient := singleton.GetRedisClient()
	capacity := 10
	refillRate := 1

	ctx := context.Background()
	laptopServer := service.NewLaptopServer(service.NewInMemoryLaptopStore(singleton.GetRedisClient()))

	log.Println("ok")

	// Start token refill in the background
	bucket, err := NewTokenBucketRateLimiter(ctx, redisClient, capacity, refillRate)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(&bucket)

	// create a gprc server to listen
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				bucket.UnaryServerInterceptor,
			),
		),
		grpc.MaxRecvMsgSize(16*1024*1024),
	)

	pb.RegisterLaptopServiceServer(grpcServer, laptopServer)
	listener, err := net.Listen("tcp", getAddressFromCMD())
	if err != nil {
		panic(err)
	}

	// start the server
	if err := grpcServer.Serve(listener); err != nil {
		panic(err)
	}

}

func getAddressFromCMD() (address string) {
	port := flag.Int("port", 0, "the server port")
	flag.Parse() // This is crucial to parse the command line arguments

	if *port == 0 {
		log.Fatal("Port must be set")
	}

	address = fmt.Sprintf("0.0.0.0:%d", *port)
	log.Println("Server is listening on: ", address)

	return
}
