package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strconv"
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
	conn, err := grpc.Dial(port,
		grpc.WithDefaultCallOptions(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
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
	tokenBucketTest()
}

type ClientMetric struct {
	TotalRequest int
	FromTime     string
	ToTime       string
}

var requestChann = make(chan time.Time, 1)
var metricsMutex sync.Mutex
var fileMutex sync.Mutex

type TestComponent struct {
	Title                          string
	Conn                           []Conn
	TimeSpanInSecond               int
	NumberOfRequestSendInTimeSpand int
	TestDurationInMinute           int

	InitialLoadInMinute           int
	MaxLoadInMinute               int
	LoadIncreaseIntervalInMinutes int
	NumberOfLoadIncreases         int

	SpikeIntervalInMinute  int
	NormalIntervalInMinute int

	// This should specify the number of requests to send during each high traffic spike.
	//  A higher value simulates a more intense spike in traffic.
	SpikeRequestCount int

	// This specifies the number of requests to send during each normal traffic interval.
	// This would typically be lower than SpikeRequestCount to simulate a more typical, moderate traffic flow.
	NormalRequestCount int
}

func tokenBucketTest() {
	log.Println("Starting token bucket test")

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

	testComponent1 := TestComponent{
		Title:                          "Token Bucket Test",
		Conn:                           conns,
		TimeSpanInSecond:               5,
		NumberOfRequestSendInTimeSpand: 60, // in 1 minute, there will be (60/TimeSpanInSecond) * NumberOfRequestSendInTimeSpand requests send to the servers.
		TestDurationInMinute:           10,
	}

	// testComponent2 := TestComponent{
	// 	Title:                         "Token Bucket Test",
	// 	Conn:                          conns,
	// 	TimeSpanInSecond:              5,
	// 	InitialLoadInMinute:           60,
	// 	NumberOfLoadIncreases:         5,
	// 	LoadIncreaseIntervalInMinutes: 1,
	// 	TestDurationInMinute:          3,
	// 	MaxLoadInMinute:               60,
	// }

	// testComponent3 := TestComponent{
	// 	Title:                  "Token Bucket Test",
	// 	Conn:                   conns,
	// 	TestDurationInMinute:   2,
	// 	SpikeIntervalInMinute:  1,
	// 	NormalIntervalInMinute: 1,
	// 	SpikeRequestCount:      10,
	// 	NormalRequestCount:     5,
	// }

	go processMetrics()

	runTest1(testComponent1)
	// runTest2(testComponent2)
	// runTest3(testComponent3)
	// runTest4(testComponent)
	// runTest5(testComponent)
	// runTest6(testComponent)
}

func runTest1(testComponent TestComponent) {

	/*	1.Objective: Assess the rate limiter's impact under low traffic conditions.

		Approach: Measure the metrics with the rate limiter enabled under a low number of requests.

		Parameters:

		Number of Requests: Low, to represent minimal traffic.
		Duration: Similar to the baseline measurement.. */

	// initially, all of the tokens are available and be tick periodically according to the refill rate.
	// thus, I will run the below-10 test-case first, this should work fine

	fmt.Println("Start test case 1")
	var wg sync.WaitGroup

	endTime := time.Now().Add(time.Duration(testComponent.TestDurationInMinute) * time.Minute)
	ticker := time.NewTicker(time.Duration(testComponent.TimeSpanInSecond) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if time.Now().After(endTime) {
				wg.Wait() // Wait for all goroutines to finish
				return    // Stop the test after Z minutes
			}

			for i := 0; i < testComponent.NumberOfRequestSendInTimeSpand; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()

					// Randomly select a server
					randomIndex := rand.Intn(len(testComponent.Conn))
					selectedConn := testComponent.Conn[randomIndex]
					cli := pb.NewLaptopServiceClient(selectedConn.Conn)

					log.Printf("start requesting: %d ----to server: %s\n", idx, selectedConn.Title)
					createLaptop(cli, fmt.Sprintf("%d-%s", idx, uuid.NewString()))

					// Send a signal to the metrics processor
					requestChann <- time.Now()
				}(i)
			}

		case <-time.After(endTime.Sub(time.Now())):
			// Stop the ticker and wait for ongoing requests
			ticker.Stop()
			wg.Wait()
			return
		}
	}
}

func processMetrics() {
	log.Println("Starting metrics processor")

	var metricsData ClientMetric
	ticker := time.NewTicker(20 * time.Second)

	for {
		select {
		case metric := <-requestChann:

			metricsMutex.Lock()
			metricsData.TotalRequest++
			if metricsData.TotalRequest == 1 {
				metricsData.FromTime = metric.Format("2006-01-02 15:04:05")
			}
			metricsMutex.Unlock()

		case <-ticker.C:

			metricsMutex.Lock()
			metricsData.ToTime = time.Now().Format("2006-01-02 15:04:05")
			metricsMutex.Unlock()

			// Process and output the metrics data
			go appendMetricsToCSV(metricsData)
			metricsData = ClientMetric{} // Reset the slice for the next interval
		}
	}
}

func runTest2(testComponent TestComponent) {
	fmt.Println("Starting test case 2, wait for 10 seconds to refill the token")
	time.Sleep(10 * time.Second)

	// Test Case 2: Request Allow Periodically Over Time
	// The server refills at the rate of 1 token per second.
	fmt.Println("Start test case 2")

	var wg sync.WaitGroup
	// for _, conn := range testComponent.Conn {
	// 	cli := pb.NewLaptopServiceClient(conn.Conn)
	// 	for i := 0; i < 100; i++ {
	// 		wg.Add(1)
	// 		go func(c Conn, idx int) {
	// 			defer wg.Done()
	// 			// Wait for a period based on the refill rate to ensure token availability
	// 			time.Sleep(time.Millisecond * 1000 * time.Duration(idx))

	// 			log.Println("start requesting: ", idx, "----to server :", c.Title)
	// 			createLaptop(cli, fmt.Sprintf("%d-%s", idx, uuid.NewString()))
	// 		}(conn, i)
	// 	}
	// }

	// wg.Wait()

	/*
		*High Traffic Scenario. Objective: Evaluate how the rate limiter copes with high traffic,
			particularly whether it introduces significant latency or CPU overhead.

		Approach: Gradually increase the load until it reaches a high level, monitoring how the rate limiter's performance changes.

		Parameters:

		Number of Requests: High, to simulate peak or stress conditions.
		Duration: Sustained period of high load, e.g., 20-30 minutes.
	*/

	endTime := time.Now().Add(time.Duration(testComponent.TestDurationInMinute) * time.Minute)
	increaseLoadTicker := time.NewTicker(time.Duration(testComponent.LoadIncreaseIntervalInMinutes) * time.Minute)
	requestTicker := time.NewTicker(time.Duration(testComponent.TimeSpanInSecond) * time.Second)
	defer requestTicker.Stop()
	defer increaseLoadTicker.Stop()

	currentLoad := testComponent.InitialLoadInMinute
	loadIncrement := (testComponent.MaxLoadInMinute - testComponent.InitialLoadInMinute) / testComponent.NumberOfLoadIncreases

	for {
		select {
		case <-increaseLoadTicker.C:
			if currentLoad < testComponent.MaxLoadInMinute {
				currentLoad += loadIncrement
				log.Printf("Increasing load to %d requests per timespan\n", currentLoad)
			}

		case <-requestTicker.C:
			if time.Now().After(endTime) {
				wg.Wait() // Wait for all goroutines to finish
				return    // Stop the test after Z minutes
			}

			for i := 0; i < currentLoad; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()

					// Randomly select a server
					randomIndex := rand.Intn(len(testComponent.Conn))
					selectedConn := testComponent.Conn[randomIndex]
					cli := pb.NewLaptopServiceClient(selectedConn.Conn)

					log.Printf("start requesting: %d ----to server: %s\n", idx, selectedConn.Title)
					createLaptop(cli, fmt.Sprintf("%d-%s", idx, uuid.NewString()))
				}(i)
			}

		case <-time.After(endTime.Sub(time.Now())):
			// Stop the tickers and wait for ongoing requests
			requestTicker.Stop()
			increaseLoadTicker.Stop()
			wg.Wait()
			return
		}
	}
}

func runTest3(testComponent TestComponent) {
	// Test Case 3: Handling Burst Traffic
	// fmt.Println("Start test case 3: Burst Traffic")

	// var wg sync.WaitGroup
	// for _, conn := range testComponent.Conn {
	// 	cli := pb.NewLaptopServiceClient(conn.Conn)
	// 	for i := 0; i < 100; i++ { // Simulate a burst of 100 requests
	// 		wg.Add(1)
	// 		go func(c Conn, idx int) {
	// 			defer wg.Done()
	// 			log.Println("Burst request: ", idx, "----to server :", c.Title)
	// 			createLaptop(cli, fmt.Sprintf("%d-%s", idx, uuid.NewString()))
	// 		}(conn, i)
	// 	}
	// }
	// wg.Wait()
	// fmt.Println("Burst traffic test completed")
	// fmt.Println("----------------------------------------------------")

	var wg sync.WaitGroup

	spikeTicker := time.NewTicker(time.Duration(testComponent.SpikeIntervalInMinute) * time.Minute)
	normalTicker := time.NewTicker(time.Duration(testComponent.NormalIntervalInMinute) * time.Minute)
	endTime := time.Now().Add(time.Duration(testComponent.TestDurationInMinute) * time.Minute)

	// Start with normal traffic
	sendNormalTraffic(&wg, testComponent)

	for {
		select {
		case <-spikeTicker.C:
			log.Println("Starting high traffic spike")
			sendSpikeTraffic(&wg, testComponent)

		case <-normalTicker.C:
			log.Println("Resuming normal traffic")
			sendNormalTraffic(&wg, testComponent)

		case <-time.After(endTime.Sub(time.Now())):
			// Stop the test
			spikeTicker.Stop()
			normalTicker.Stop()
			wg.Wait()
			return
		}
	}
}

func sendSpikeTraffic(wg *sync.WaitGroup, testComponent TestComponent) {
	for i := 0; i < testComponent.SpikeRequestCount; i++ {
		wg.Add(1)
		go sendRequest(wg, testComponent)
	}
}

func sendNormalTraffic(wg *sync.WaitGroup, testComponent TestComponent) {
	for i := 0; i < testComponent.NormalRequestCount; i++ {
		wg.Add(1)
		go sendRequest(wg, testComponent)
	}
}

func sendRequest(wg *sync.WaitGroup, testComponent TestComponent) {
	defer wg.Done()

	// Randomly select a server
	randomIndex := rand.Intn(len(testComponent.Conn))
	selectedConn := testComponent.Conn[randomIndex]
	cli := pb.NewLaptopServiceClient(selectedConn.Conn)

	log.Printf("Sending request to server: %s\n", selectedConn.Title)
	createLaptop(cli, fmt.Sprintf("%s", uuid.NewString()))
}

func runTest4(testComponent TestComponent) {
	// Test Case 4: High Load Over Extended Period
	fmt.Println("Start test case 4: High Load Over Extended Period")

	var wg sync.WaitGroup
	duration := time.Minute * 10 // Run for 10 minutes
	end := time.Now().Add(duration)
	for time.Now().Before(end) {
		for _, conn := range testComponent.Conn {
			cli := pb.NewLaptopServiceClient(conn.Conn)
			wg.Add(1)
			go func(c Conn) {
				defer wg.Done()
				createLaptop(cli, uuid.NewString())
			}(conn)
		}
		time.Sleep(1 * time.Second) // Adjust as needed
	}
	wg.Wait()
	fmt.Println("Extended high load test completed")
	fmt.Println("----------------------------------------------------")

}

func runTest5(testComponent TestComponent) {
	// Test Case 5: Randomized Request Timing
	fmt.Println("Start test case 5: Randomized Request Timing")

	var wg sync.WaitGroup
	for _, conn := range testComponent.Conn {
		cli := pb.NewLaptopServiceClient(conn.Conn)
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(c Conn, idx int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
				log.Println("Random timing request: ", idx, "----to server :", c.Title)
				createLaptop(cli, fmt.Sprintf("%d-%s", idx, uuid.NewString()))
			}(conn, i)
		}
	}
	wg.Wait()
	fmt.Println("Randomized request timing test completed")
	fmt.Println("----------------------------------------------------")
}

func runTest6(testComponent TestComponent) {
	// Test Case 6: Accuracy Test
	fmt.Println("Start test case 6: Accuracy Test")

	var wg sync.WaitGroup
	allowedCount := 0
	rejectedCount := 0
	for _, conn := range testComponent.Conn {
		cli := pb.NewLaptopServiceClient(conn.Conn)
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(c Conn, idx int) {
				defer wg.Done()
				// Adjust the sleep time based on expected rate limiting
				time.Sleep(time.Duration(idx%10) * 100 * time.Millisecond)
				err := createLaptop(cli, fmt.Sprintf("%d-%s", idx, uuid.NewString()))
				if err == nil {
					allowedCount++
				} else {
					rejectedCount++
				}
			}(conn, i)
		}
	}
	wg.Wait()
	fmt.Printf("Accuracy test completed: %d allowed, %d rejected\n", allowedCount, rejectedCount)
	fmt.Println("----------------------------------------------------")
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

func createLaptop(laptopClient pb.LaptopServiceClient, id string) error {
	laptop := sample.NewLaptop()

	req := &pb.CreateLaptopRequest{
		Laptop: laptop,
	}

	res, err := laptopClient.CreateLaptop(context.Background(), req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.AlreadyExists {
			log.Println("laptop already exist")

			return nil
		}

		return errors.New("cannot create laptop")
	}

	log.Printf("request success: %s", res.Id)
	return nil
}

func appendMetricsToCSV(metric ClientMetric) error {
	fileMutex.Lock() // one goroutine write to the file at a time
	defer fileMutex.Unlock()

	// Open the file in append mode, create it if it doesn't exist
	file, err := os.OpenFile("client_stats.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Writing the data
	if err = writer.Write([]string{
		strconv.Itoa(metric.TotalRequest),
		metric.FromTime,
		metric.ToTime,
	}); err != nil {
		return err
	}

	return nil
}
