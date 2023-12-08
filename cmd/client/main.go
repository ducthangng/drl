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
			Title:   "server 1",
			Address: "localhost:8082",
			Conn:    newConn("localhost:8082"),
		},
	}...)

	var wg sync.WaitGroup
	for _, conn := range conns {
		cli := pb.NewLaptopServiceClient(conn.Conn)
		for i := 0; i < 200; i++ {
			wg.Add(1)
			go func(c Conn, idx int) {
				defer wg.Done()
				// Sleep is generally used for simulation or debugging
				time.Sleep(time.Duration(idx) * 100 * time.Millisecond)
				log.Println("start requesting: ", idx)
				createLaptop(cli, fmt.Sprintf("%d-%s", idx, uuid.NewString()))
			}(conn, i)
		}
	}

	wg.Wait()
	fmt.Println("All create successfully")

	// wg.Add(len(conns))
	// // for _, conn := range conns {
	// // 	go func(conn Conn) {
	// // 		defer wg.Done()

	// // 		filter := &pb.Filter{
	// // 			MaxPriceUsd: 10000,
	// // 			MinCpuCore:  2,
	// // 			MinCpuGhz:   1,
	// // 			MinRam: &pb.Memory{
	// // 				Value: 2,
	// // 				Unit:  pb.Memory_GIGABYTE,
	// // 			},
	// // 		}

	// // 		searchLaptop(conn.Client, filter, conn.Title)
	// // 	}(conn)
	// // }

	// // wg.Wait()
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
			log.Println("unexpected error: ", st.Message())
		}

		return
	}

	log.Printf("created laptop with id: %s", res.Id)
}
