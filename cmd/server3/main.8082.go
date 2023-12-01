package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/ducthangng/drl/bucket"
	"github.com/ducthangng/drl/pb"
	"github.com/ducthangng/drl/service"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
)

func main() {
	// create a new laptop server
	port := flag.Int("port", 8082, "the server port")
	log.Println("Starting server on port: ", *port)

	address := fmt.Sprintf("0.0.0.0:%d", *port)

	laptopServer := service.NewLaptopServer(service.NewInMemoryLaptopStore())

	limiter := bucket.GetLimiter()
	// create a gprc server to listen
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				limiter.UnaryServerInterceptor,
			),
		),
	)

	pb.RegisterLaptopServiceServer(grpcServer, laptopServer)

	litener, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}

	// start the server
	if err := grpcServer.Serve(litener); err != nil {
		panic(err)
	}

}