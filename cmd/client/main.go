package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/ducthangng/drl/pb"
	"github.com/ducthangng/drl/sample"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func newConn(port string) *grpc.ClientConn {
	conn, err := grpc.Dial(port, grpc.WithDefaultCallOptions(
	// grpc.MaxCallRecvMsgSize(16*1024*1024),
	), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	log.Println("successfully connected to ", port)

	return conn
}

type Conn struct {
	Title   string
	Address string
	Conn    *grpc.ClientConn
	Client  pb.LaptopServiceClient
}

func main() {
	var conns []Conn
	conns = append(conns, []Conn{
		{
			Title:   "server 1",
			Address: "localhost:8081",
			Conn:    newConn("localhost:8081"),
		},
		{
			Title:   "server 2",
			Address: "localhost:8082",
			Conn:    newConn("localhost:8082"),
		},
	}...)

	// redisClient := getRedisClient()

	// Test Case 1: Request Allowed
	// Assume enough tokens are available
	// initially, all of the tokens are available and be tick periodically according to the refill rate.
	// thus, I will run the below-10 test-case first, this should work fine
	fmt.Println("Start test case 1")
	var wg sync.WaitGroup
	for _, conn := range conns {
		cli := pb.NewLaptopServiceClient(conn.Conn)
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func(c Conn, idx int) {
				defer wg.Done()
				time.Sleep(time.Duration(idx) * 100 * time.Millisecond)
				log.Println("start requesting: ", idx, "----to server :", c.Title)
				createLaptop(cli, fmt.Sprintf("%d-%s", idx, uuid.NewString()))
			}(conn, i)
		}
	}

	wg.Wait()
	fmt.Println("All create successfully")
	fmt.Println("----------------------------------------------------")
	fmt.Println("Starting test case 2, wait for 10 seconds to refill the token")
	time.Sleep(10 * time.Second)

	// Test Case 2: Request Allow Periodically Over Time
	// The server refills at the rate of 1 token per second.
	fmt.Println("Start test case 2")
	for _, conn := range conns {
		cli := pb.NewLaptopServiceClient(conn.Conn)
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(c Conn, idx int) {
				defer wg.Done()
				// Wait for a period based on the refill rate to ensure token availability
				time.Sleep(time.Millisecond * 1000 * time.Duration(idx))

				log.Println("start requesting: ", idx, "----to server :", c.Title)
				createLaptop(cli, fmt.Sprintf("%d-%s", idx, uuid.NewString()))
			}(conn, i)
		}
	}

	wg.Wait()
	fmt.Println("All requests processed")
	fmt.Println("----------------------------------------------------")

	// Test Case 3: Change the Refill Rate while Token Refill Over Time
	// write to check if the refill rate is changed
	// then, proceed to make several request to check if the request is allowed or denied

}

func searchLaptop(laptopClient pb.LaptopServiceClient, filter *pb.Filter, id string) {
	req := &pb.SearchLaptopRequest{
		Filter: filter,
	}

	stream, err := laptopClient.SearchLaptop(context.Background(), req)
	if err != nil {
		panic(err)
	}

	for {
		res, err := stream.Recv()
		if err == io.EOF {
			log.Println("no more data from", id)
			return
		}

		if status.Code(err) == codes.NotFound {
			log.Println("not found")
			return
		}

		if err != nil {
			log.Println("unexpected error: ", err)
			continue
		}

		laptop := res.GetLaptop()
		log.Printf("- found: %s from %s", laptop.Id[0:5], id)
	}
}

func createLaptop(laptopClient pb.LaptopServiceClient, id string) {
	laptop := sample.NewLaptop()

	req := &pb.CreateLaptopRequest{
		Laptop: laptop,
	}

	res, err := laptopClient.CreateLaptop(context.Background(), req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.AlreadyExists {
			log.Println("laptop already exist")
		} else {
			log.Println("request failed: ", st.Message())
		}

		return
	}

	log.Printf("request success: %s", res.Id)
}
