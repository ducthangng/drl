package bucket

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/go-zookeeper/zk"
)

type RateLimiterAddress struct {
	Address  string
	IsMaster bool
}

type RateLimiterNode struct {
	Address     string
	Mutex       sync.Mutex
	RedisClient *redis.Client
	Capacity    int
	Tokens      int
	RefillRate  int // Number of tokens to add per second

	otherNodeIDs  []string // List of node IDs with their addresses\
	isEstablished bool
	isMaster      bool
}

var ServerAddress = []string{ // default, can be changed to be dynamics
	"localhost:2181",
	"localhost:2182",
	"localhost:2183",
}

const (
	// FlagEphemeral means the node is ephemeral.
	FlagEphemeral = 1
	FlagSequence  = 2
	FlagTTL       = 4
)

// NewDistributedRateLimiter creates a new instance of DistributedRateLimiter
func NewDistributedRateLimiter(ctx context.Context, redisClient *redis.Client, capacity, refillRate int, currAddress string) (*RateLimiterNode, error) {
	if len(currAddress) == 0 {
		return nil, ErrorEmptyAddress
	}

	rateLimiter := &RateLimiterNode{
		RedisClient: redisClient,
		Capacity:    capacity,
		Tokens:      capacity,
		RefillRate:  refillRate,
		Address:     currAddress,
	}

	if err := rateLimiter.Initialize(ctx); err != nil {
		return nil, err
	}

	return rateLimiter, nil
}

func (r *RateLimiterNode) Initialize(ctx context.Context) (err error) {
	// Check a central service registry or configuration to see if this node is the master
	// For simplicity, let's assume a function that checks this
	if err = r.RedisClient.SAdd(context.Background(), "@admin_rate_limiter_node", r.Address).Err(); err != nil {
		return
	}

	go r.initializeZookeeperConnection()
	go r.refillTokens()

	return
}

func (r *RateLimiterNode) initializeZookeeperConnection() {
	// Just connect, not watching, thus channel is unused
	c, _, err := zk.Connect(ServerAddress, time.Second*10) //second from losing connection to re-establishing a new connection
	if err != nil {
		panic(err)
	}

	// check if the persistent node /app/leader_election exists
	exists, _, err := c.Exists("/leader_election")
	if err != nil {
		log.Println("zk.Exists failed x", err)
		panic(err)
	}

	if !exists {
		// create a folder for leader election
		_, err = c.Create("/leader_election", []byte("rate_limit"), 0, zk.WorldACL(zk.PermAll))
		if err != nil {
			log.Println("zk.Exists failed xx", err)
			panic(err)
		}

		log.Println("created /leader_election")
	}

	children, _, err := c.Children("/leader_election")
	if err != nil {
		log.Println("zk.Exists failed yy", err)
		panic(err)
	}

	log.Println("found existed nodes: ", children)

	// children is named guid_0000000001, guid_0000000002, ...
	// find the largest one
	largest := -1
	for _, child := range children {
		splitted := strings.Split(child, "_")
		if len(splitted) != 2 || splitted[0] != "guid" {
			panic("invalid child name" + child)
		}

		id := splitted[1]
		if len(id) != 10 {
			panic("invalid child name" + child)
		}

		x, err := strconv.Atoi(id)
		if err != nil {
			panic(err)
		}

		if x > largest {
			largest = x
		}
	}

	// increase the largest one by 1
	myId := fmt.Sprintf("guid_%010d", largest+1)

	// create the node
	_, err = c.Create("/leader_election/"+myId, []byte("rate_limit"), FlagEphemeral, zk.WorldACL(zk.PermAll))
	if err != nil {
		panic(err)
	}

	fmt.Println("created node successfully: ", myId)
	go ticker(myId)

	watchId := fmt.Sprintf("%010d", largest)

	// !CRITICAL: this is the key to watch the node

	r.Mutex.Lock()

	r.otherNodeIDs = children
	if largest == -1 {
		watchId = fmt.Sprintf("%010d", largest+1)
		r.isMaster = true
	}

	r.isEstablished = true
	r.Mutex.Unlock()

	log.Println("watching node id: ", watchId)

	go func() {
		var ch <-chan zk.Event
		for {
			_, _, ch, err = c.GetW(fmt.Sprintf("/leader_election/guid_%s", watchId))
			if err != nil {
				panic("Failed to watch node: " + err.Error())
			}

			e := <-ch
			fmt.Printf("%+v\n", e)

			if e.Type == zk.EventNodeDeleted {
				log.Println("Master Node Is Deleted:", watchId)

				var data []byte
				var nch <-chan zk.Event
				data, _, nch, err = c.GetW(fmt.Sprintf("/leader_election/%s", myId))
				if err != nil {
					panic(err)
				}
				log.Println("watching it self", string(data))

				r.Mutex.Lock()
				r.isMaster = true
				r.Mutex.Unlock()

				ex := <-nch
				fmt.Printf("%+v\n", ex.Err.Error())
			}
		}
	}()

}

func ticker(myId string) {
	ticker := time.NewTicker(time.Second * 3)
	for {
		select {
		case <-ticker.C:
			log.Println(myId + " online")
		}
	}
}

// func (n *RateLimiterNode) checkIfMaster(ctx context.Context, nodeID string) error {
// 	// Implement logic to check if this node should be the master
// 	// The rule is: if no existed node in the cluster, the first node will be the master
// 	// Thus, simply check if no nodes existed in the cluster, this node will be the master
// 	// !PROBLEM: what if 2 nodes are created at the same time?
// 	// !SOLUTION: use a central service registry to check if there is any node existed, use a lock to ensure only one node can be created at a time
// 	// For now, query the master node to get its addrees
// 	masterNodeAddress, err := n.RedisClient.Get(context.Background(), "@admin_rate_limiter_node_address").Result()
// 	if err != nil {
// 		panic(err)
// 	}

// 	switch len(masterNodeAddress) {
// 	case 0:
// 		n.isMaster = true
// 		n.masterAddr = n.Address
// 		n.RedisClient.Set(context.Background(), "@admin_rate_limiter_node_address", n.Address, 0)

// 	case 1:
// 		n.masterAddr = masterNodeAddress

// 	default:
// 		panic("multiple master nodes")
// 	}

// 	// If there are other nodes, this node is not the master
// 	// find the master node and assign it to the master address
// 	return nil
// }

// func (n *RateLimiterNode) checkMasterHealth(ctx context.Context) {
// 	// This is a simplified version. You would need to integrate actual network calls here.
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return // Exit if the context is cancelled
// 		case <-time.After(5 * time.Second): // Check every 5 seconds, for example
// 			if n.isMaster {
// 				// If this node is the master, it's always "healthy" from its own perspective
// 				continue
// 			}

// 			// If not the master, check the master's health
// 			healthy := n.pingMaster(ctx)
// 			if !healthy {
// 				log.Println("Master is not healthy, initiating master election process...")
// 				n.electNewMaster() // Implement this method for the master election process
// 			}
// 		}
// 	}
// }

// func (n *RateLimiterNode) pingMaster(ctx context.Context) bool {
// 	// grpc call to server address
// 	// return true if healthy
// 	masterAddress := n.masterAddr
// 	conn, err := grpc.Dial(masterAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
// 	if err != nil {
// 		log.Printf("Failed to dial master: %v", err)
// 		return false
// 	}
// 	defer conn.Close()

// 	client := pb.NewRateLimiterServiceClient(conn)

// 	// Call the PingMaster method
// 	response, err := client.PingMaster(ctx, &pb.PingRequest{
// 		ClientId: n.masterAddr,
// 	})

// 	if err != nil {
// 		log.Printf("Failed to ping master: %v", err)
// 		return false
// 	}

// 	return response.IsHealthy
// }

// func (n *RateLimiterNode) electNewMaster() {
// 	n.Mutex.Lock()
// 	defer n.Mutex.Unlock()

// 	if n.isMaster {
// 		// The current node is already the master
// 		return
// 	}

// 	// Sort the node IDs to maintain a consistent order
// 	sortedNodeIDs := make([]string, len(n.otherNodeIDs))
// 	sort.Strings(sortedNodeIDs)

// 	// Find the next master
// 	for _, id := range sortedNodeIDs {
// 		if id > n.Address {
// 			// Found the next node in the order
// 			n.assignNewMaster(id)
// 			return
// 		}
// 	}

// 	// If no higher ID is found, wrap around to the lowest ID
// 	if len(sortedNodeIDs) > 0 {
// 		n.assignNewMaster(sortedNodeIDs[0])
// 	}
// }

// func (n *RateLimiterNode) assignNewMaster(newMasterAddress string) {
// 	// Set up a gRPC client and connect to the new master node
// 	// For simplicity, let's assume you have a function to get the address of a node by its ID
// 	if len(newMasterAddress) == 0 {
// 		panic("new master id is empty")
// 	}

// 	conn, err := grpc.Dial(newMasterAddress, grpc.WithTransportCredentials(insecure.NewCredentials())) // Use secure options in production
// 	if err != nil {
// 		log.Fatalf("Failed to dial: %v", err)
// 	}
// 	defer conn.Close()

// 	client := pb.NewRateLimiterServiceClient(conn)

// 	// Call the AssignNewMaster method
// 	_, err = client.AssignNewMaster(context.Background(), &pb.AssignMasterRequest{
// 		NewMasterID: newMasterAddress,
// 	})
// 	if err != nil {
// 		log.Fatalf("Failed to assign new master: %v", err)
// 	}

// 	// Update the master-related state in the current node
// 	// ...
// }
