package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/ducthangng/drl/pb"
	"github.com/ducthangng/drl/sample"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func newConn(port string) *grpc.ClientConn {
	conn, err := grpc.Dial(port, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

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
			Address: "localhost:8080",
			Conn:    newConn("localhost:8080"),
		}, {
			Title:   "server 2",
			Address: "localhost:8081",
			Conn:    newConn("localhost:8081"),
		}, {
			Title:   "server 3",
			Address: "localhost:8082",
			Conn:    newConn("localhost:8082"),
		},
	}...)

	for i := range conns {
		conns[i].Client = pb.NewLaptopServiceClient(conns[i].Conn)
	}

	count := 0
	for i, conn := range conns {

		id := string(i) + string(count)

		for i := 0; i < 10; i++ {
			count++
			createLaptop(conn.Client, id)
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(conns))

	for _, conn := range conns {
		go func(conn Conn) {
			defer wg.Done()

			filter := &pb.Filter{
				MaxPriceUsd: 10000,
				MinCpuCore:  2,
				MinCpuGhz:   1,
				MinRam: &pb.Memory{
					Value: 2,
					Unit:  pb.Memory_GIGABYTE,
				},
			}

			searchLaptop(conn.Client, filter, conn.Title)
		}(conn)
	}

	wg.Wait()
	fmt.Println("All requests completed")
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
			panic(err)
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
			log.Println("unexpected error: ", st.Message())
		}

		return
	}

	log.Printf("created laptop with id: %s", res.Id)
}
