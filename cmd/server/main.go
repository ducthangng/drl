package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ducthangng/drl/pb"
	"github.com/ducthangng/drl/rl/entity"
	"github.com/ducthangng/drl/rl/token_bucket"
	"github.com/ducthangng/drl/service"
	"github.com/ducthangng/drl/singleton"
	"github.com/go-redis/redis/v8"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/memory"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TokenBucketRateLimiter struct {
	bucket *token_bucket.RateLimiterNode
}

type MetricsData struct {
	Latencies      []time.Duration
	RequestDenied  []string  // request ID
	RequestHandled []string  // request ID
	CPUUsage       []float64 // CPU usage in percentage
	MemoryUsage    []float64 // Memory usage in percentage
}

type Metric struct {
	Latency  time.Duration
	Response time.Duration
	ID       string
	IsAllow  bool
	FromTime string
	ToTime   string
	CPUUsage float64
	MemUsage float64
}

var metrics MetricsData
var metricsMutex sync.Mutex
var fileMutex sync.Mutex
var latencyMetricsChannel = make(chan Metric, 100)
var repsponseMetricsChannel = make(chan Metric, 100)
var capacity = 60
var refillRatePerSecond = 1
var startCPU *cpu.Stats
var startMem *memory.Stats

func initializeMem() {
	var err error

	startCPU, err = cpu.Get()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}

	startMem, err = memory.Get()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}

	return
}

func main() {
	redisClient := singleton.GetRedisClient()
	ctx := context.Background()

	redisClient.Ping(ctx)
	initializeMem()

	laptopServer := service.NewLaptopServer(service.NewInMemoryLaptopStore(singleton.GetRedisClient()))

	// Start token refill in the background
	// bucket, err := NewTokenBucketRateLimiter(ctx, redisClient, capacity, refillRatePerSecond)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	go processMetrics()

	// create a gprc server to listen
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				UnaryServerInterceptorWithMetrics,
			),
		),
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

func UnaryServerInterceptorWithMetrics(
	ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	startTime := time.Now()

	resp, err := handler(ctx, req)
	// End time after handler execution
	finished := time.Now()

	isHandled := true
	if err != nil {
		isHandled = false
	}

	go addLatency(Metric{
		Latency:  0,                       // Latency  = 0
		Response: finished.Sub(startTime), // Same as total latency in this case
		IsAllow:  isHandled,
		ID:       (req.(*pb.CreateLaptopRequest)).Laptop.Id,
		FromTime: startTime.Format("2006-01-02 15:04:05"),
	})

	return resp, err

}

// Middleware is a middleware function for HTTP handlers to enforce rate limiting
func (t *TokenBucketRateLimiter) UnaryServerInterceptorWithTokenBucketEnabled(
	ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

	// Attempt to get a token from the bucket
	startTime := time.Now()

	allowed := t.bucket.Try(ctx, entity.Request{})

	endTime := time.Now()

	// all the nodes consume the token, even the master
	// this mean that master will do extra work: to refills the tokens.

	// Check if the request is allowed by the rate limiter
	if allowed {
		// Invoke the handler
		resp, err := handler(ctx, req)

		// End time after handler execution
		finished := time.Now()

		// Send latency metrics (including handler response time)

		go addLatency(Metric{
			Latency:  endTime.Sub(startTime),  // Total response time
			Response: finished.Sub(startTime), // Same as total latency in this case
			IsAllow:  allowed,
			ID:       (req.(*pb.CreateLaptopRequest)).Laptop.Id,
			FromTime: startTime.Format("2006-01-02 15:04:05"),
		})

		return resp, err
	} else {
		// For requests that are not allowed, capture the time and send metrics
		endTime := time.Now()
		go addLatency(Metric{
			Latency:  endTime.Sub(startTime),
			Response: 0, // No handler response time as the request was not allowed
			IsAllow:  allowed,
			ID:       (req.(*pb.CreateLaptopRequest)).Laptop.Id,
			FromTime: startTime.Format("2006-01-02 15:04:05"),
		})

		return nil, status.Error(codes.ResourceExhausted, "Rate limit exceeded")
	}
}

func addLatency(metric Metric) {
	latencyMetricsChannel <- metric
}

func processMetrics() {
	log.Println("Starting metrics processor")

	var metricsData []Metric
	ticker := time.NewTicker(20 * time.Second)

	for {
		select {
		case metric := <-latencyMetricsChannel:

			metricsMutex.Lock()

			cpuUsage, err := cpu.Get()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				return
			}

			memUsage, err := memory.Get()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				return
			}

			metric.CPUUsage = float64(cpuUsage.User)
			metric.MemUsage = float64(memUsage.Used)

			metricsData = append(metricsData, metric)
			metricsMutex.Unlock()

		case <-ticker.C:

			// Process and output the metrics data
			go appendMetricsToCSV(metricsData)
			metricsData = nil // Reset the slice for the next interval
		}
	}
}

func appendMetricsToCSV(metrics []Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	fileMutex.Lock() // one goroutine write to the file at a time
	defer fileMutex.Unlock()

	// Open the file in append mode, create it if it doesn't exist
	file, err := os.OpenFile("stats.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Writing the data
	averageLatency := 0
	acceptedRequest := 0
	averageResponse := 0
	deniedRequest := 0
	averageCPUUsage := 0.0
	averageMemUsage := 0.0

	for _, metric := range metrics {
		latencyInMs := metric.Latency.Milliseconds()
		averageLatency += int(latencyInMs)

		responsInMs := metric.Response.Milliseconds()
		averageResponse += int(responsInMs)

		averageCPUUsage += metric.CPUUsage
		averageMemUsage += metric.MemUsage

		if metric.IsAllow {
			acceptedRequest++
		} else {
			deniedRequest++
		}
	}

	averageLatency = averageLatency / len(metrics)
	averageResponse = averageResponse / len(metrics)
	averageCPUUsage = (averageCPUUsage / float64(len(metrics))) - float64(startCPU.User)
	averageMemUsage = (averageMemUsage / float64(len(metrics))) - float64(startMem.Used)

	record := []string{metrics[0].FromTime, metrics[len(metrics)-1].FromTime,
		strconv.FormatInt(int64(averageLatency), 10), strconv.FormatInt(int64(averageResponse), 10),
		strconv.Itoa(acceptedRequest), strconv.Itoa(deniedRequest), strconv.Itoa(len(metrics)), strconv.FormatFloat(averageCPUUsage/float64(len(metrics)), 'f', 6, 64), strconv.FormatFloat(averageMemUsage/float64(len(metrics)), 'f', 6, 64)}
	if err := writer.Write(record); err != nil {
		return err
	}

	return nil
}

func NewTokenBucketRateLimiter(ctx context.Context, redisClient *redis.Client, capacity int, refillRate int) (*TokenBucketRateLimiter, error) {
	b, err := token_bucket.NewDistributedRateLimiter(ctx, redisClient, capacity, refillRate)
	if err != nil {
		return nil, err
	}

	return &TokenBucketRateLimiter{
		bucket: b,
	}, nil
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
