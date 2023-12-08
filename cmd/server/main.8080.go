package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/ducthangng/drl/pb"
	"github.com/ducthangng/drl/service"
	"github.com/ducthangng/drl/singleton"
	"github.com/ducthangng/drl/sliding_log"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
)

func main() {
	laptopServer := service.NewLaptopServer(service.NewInMemoryLaptopStore(singleton.GetRedisClient()))

	// Start token refill in the background
	// tokenBucket := bucket.GetLimiter()
	slidingWindow := sliding_log.GetLimiter()
	// log.Println(&tokenBucket)
	log.Println(&slidingWindow)

	// create a gprc server to listen
	grpcServer := grpc.NewServer(
		// grpc.UnaryInterceptor(
		// 	grpc_middleware.ChainUnaryServer(
		// 		tokenBucket.UnaryServerInterceptor,
		// 	),
		// ),
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				slidingWindow.UnaryServerInterceptor(),
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
