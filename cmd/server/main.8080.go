package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/ducthangng/drl/rl/bucket"
	"github.com/ducthangng/drl/singleton"
)

func main() {

	fmt.Println("Hello world")

	redisClient := singleton.GetRedisClient()
	capacity := 10
	refillRate := 1

	ctx := context.Background()
	addr := getAddressFromCMD()

	_, err := bucket.NewDistributedRateLimiter(ctx, redisClient, capacity, refillRate, addr)
	if err != nil {
		log.Fatal(err)
	}

	// laptopServer := service.NewLaptopServer(service.NewInMemoryLaptopStore(singleton.GetRedisClient()))

	// // Start token refill in the background
	// // tokenBucket := bucket.GetLimiter()
	// slidingWindow := sliding_log.GetLimiter()
	// // log.Println(&tokenBucket)
	// log.Println(&slidingWindow)

	// // create a gprc server to listen
	// grpcServer := grpc.NewServer(
	// 	// grpc.UnaryInterceptor(
	// 	// 	grpc_middleware.ChainUnaryServer(
	// 	// 		tokenBucket.UnaryServerInterceptor,
	// 	// 	),
	// 	// ),
	// 	grpc.UnaryInterceptor(
	// 		grpc_middleware.ChainUnaryServer(
	// 			slidingWindow.UnaryServerInterceptor(),
	// 		),
	// 	),
	// 	grpc.MaxRecvMsgSize(16*1024*1024),
	// )

	// pb.RegisterLaptopServiceServer(grpcServer, laptopServer)
	// listener, err := net.Listen("tcp", getAddressFromCMD())
	// if err != nil {
	// 	panic(err)
	// }

	// // start the server
	// if err := http.ListenAndServe(listener, ); err != nil {
	// 	panic(err)
	// }
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, you've requested: %s", r.URL.Path)
	})

	// Start the HTTP server with the handler
	fmt.Println("Starting server at", addr)
	err = http.ListenAndServe(addr, nil) // 'nil' uses the default handler, which we've defined above
	if err != nil {
		panic(err)
	}

}

func getAddressFromCMD() (address string) {
	pr := flag.Int("prt", 0, "the server port")
	flag.Parse() // This is crucial to parse the command line arguments

	if *pr == 0 {
		log.Fatal("Port must be set")
	}

	address = fmt.Sprintf("0.0.0.0:%d", *pr)
	log.Println("Server is listening on: ", address)

	return
}
