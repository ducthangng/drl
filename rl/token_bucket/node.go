package token_bucket

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ducthangng/drl/rl/entity"
	"github.com/go-redis/redis/v8"
	"github.com/go-zookeeper/zk"
)

type RateLimiterNode struct {
	ID               string
	Mutex            sync.Mutex
	RedisClient      *redis.Client
	Capacity         int
	Tokens           int
	RefillRate       int // Number of tokens to add per second
	Conn             *zk.Conn
	NodeChan         <-chan zk.Event
	IntervalNodeChan <-chan zk.Event
	TickerChanged    chan bool

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
	FlagConsistent = 0
	FlagEphemeral  = 1
	FlagSequence   = 2
	FlagTTL        = 4
)

// NewDistributedRateLimiter creates a new instance of DistributedRateLimiter
func NewDistributedRateLimiter(ctx context.Context, redisClient *redis.Client, capacity, refillRate int) (*RateLimiterNode, error) {
	rateLimiter := &RateLimiterNode{
		RedisClient:   redisClient,
		Capacity:      capacity,
		Tokens:        capacity,
		RefillRate:    refillRate,
		TickerChanged: make(chan bool, 1),
	}

	if err := rateLimiter.Initialize(ctx); err != nil {
		return nil, err
	}

	return rateLimiter, nil
}

func (r *RateLimiterNode) Initialize(ctx context.Context) (err error) {
	// Check a central service registry or configuration to see if this node is the master
	// For simplicity, let's assume a function that checks this

	r.initializeZookeeperConnection() // this is the key to watch the node
	go r.refillTokens()

	return
}

func (r *RateLimiterNode) initializeZookeeperConnection() {
	// Just connect, not watching, thus channel is unused

	r.Mutex.Lock()

	c, _, err := zk.Connect(ServerAddress, time.Second*10) //second from losing connection to re-establishing a new connection
	if err != nil {
		panic(err)
	}

	// check if the persistent node /app/leader_election exists
	exists, _, err := c.Exists(entity.ZNODE_PATH)
	if err != nil {
		log.Println("zk.Exists failed x", err)
		panic(err)
	}

	if !exists {
		// create a folder for leader election
		_, err = c.Create(entity.ZNODE_PATH, entity.ZNODE_CONTENT, FlagConsistent, zk.WorldACL(zk.PermAll))
		if err != nil {
			log.Println("zk.Exists failed xx", err)
			panic(err)
		}

		log.Println("created /leader_election")
	}

	// create a znode call INTERVAL_NODE to store the refill rate
	exists, _, err = c.Exists(entity.INTERVAL_NODE_PATH)
	if err != nil {
		log.Println("zk.Exists failed x", err)
		panic(err)
	}

	if !exists {
		_, err = c.Create(entity.INTERVAL_NODE_PATH, []byte(strconv.Itoa(r.RefillRate)), FlagConsistent, zk.WorldACL(zk.PermAll))
		if err != nil {
			log.Println("zk.Exists failed xxx", err)
			panic(err)
		}

		log.Println("created ", entity.INTERVAL_NODE_PATH)
	} else {
		// if the node already existed, get the
		data, _, err := c.Get(entity.INTERVAL_NODE_PATH)
		if err != nil {
			panic(err)
		}

		r.RefillRate, err = strconv.Atoi(string(data))
		if err != nil {
			panic(err)
		}

		log.Println("got refill rate: ", r.RefillRate)
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
			// this is the interval node
			continue
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
	myId := strings.Join([]string{entity.ZNODE_PATH, fmt.Sprintf("guid_%010d", largest+1)}, "/")

	// create the node
	_, err = c.Create(myId, entity.ZNODE_CONTENT, FlagEphemeral, zk.WorldACL(zk.PermAll))
	if err != nil {
		panic(err)
	}

	fmt.Println("created node successfully: ", (myId))
	go ticker(myId)

	watchId := fmt.Sprintf("%010d", largest)

	// !CRITICAL: this is the key to watch the node

	r.ID = myId
	r.otherNodeIDs = children
	isFirstNode := largest == -1
	if isFirstNode {
		r.isMaster = true
		watchId = fmt.Sprintf("%010d", largest+1)
		log.Println("I am master!")
	}

	r.isEstablished = true
	r.Conn = c

	r.Mutex.Unlock()

	if isFirstNode {
		// If this node is the first node, it should immediately watch the INTERVAL_NODE
		go r.watchIntervalNodeContinuously()
	}

	log.Println("watching node id: ", watchId)

	// Watch logic
	go func() {
		if !isFirstNode {
			_, _, nodeChan, err := r.Conn.GetW(getNodeID(watchId))
			if err != nil {
				panic("Failed to watch node: " + err.Error())
			}
			e := <-nodeChan
			fmt.Printf("%+v\n", e)

			if e.Type == zk.EventNodeDeleted {
				// Watching itself
				log.Println("watching itself at id: ", r.ID)
				_, _, r.NodeChan, err = r.Conn.GetW(r.ID)
				if err != nil {
					panic(err)
				}

				r.Mutex.Lock()
				r.isMaster = true

				r.Mutex.Unlock()

				// Watching the INTERVAL_NODE
				// This part needs to be executed regardless of whether this node was initially the master

				go r.watchIntervalNodeContinuously()
			}
		}

		// Watching the INTERVAL_NODE
		// This part needs to be executed regardless of whether this node was initially the master
		// r.watchIntervalNodeContinuously()

		// // Handling events for self and INTERVAL_NODE
		// for {
		// 	select {
		// 	case ex := <-r.NodeChan:
		// 		fmt.Printf("Node event: %+v\n", ex.Err.Error())
		// 	case intervalEvent := <-r.IntervalNodeChan:
		// 		fmt.Printf("Interval node event: %+v\n", intervalEvent.Type.String())
		// 		if intervalEvent.Type == zk.EventNodeDataChanged {
		// 			r.updateRefillRate()
		// 			r.watchIntervalNode()
		// 		}
		// 	}
		// }

	}()
}

func (r *RateLimiterNode) watchIntervalNodeContinuously() {
	for {
		log.Println("Setting up watch on INTERVAL_NODE")

		// Re-establish the watch at the start of each iteration
		_, _, intervalEventChan, err := r.Conn.GetW(entity.INTERVAL_NODE_PATH)
		if err != nil {
			log.Printf("Failed to establish watch on INTERVAL_NODE: %v", err)
			time.Sleep(time.Second) // Add a delay before retrying to avoid rapid loops in case of consistent failure
			continue
		}

		// Wait for an event
		event, ok := <-intervalEventChan
		if !ok {
			log.Println("Channel closed. Re-establishing watch.")
			continue
		}

		log.Printf("Interval node event: %+v", event.Type.String())

		if event.Type == zk.EventNodeDataChanged {
			log.Println("INTERVAL_NODE data changed, updating refill rate")
			r.updateRefillRate()
			log.Println("update successfully 2")
		} else {
			log.Printf("Unhandled event type on INTERVAL_NODE: %+v", event.Type)
		}

		log.Println("update successfully")
	}
}

// updateRefillRate updates the RefillRate based on the data in INTERVAL_NODE.
func (r *RateLimiterNode) updateRefillRate() {
	log.Println("updateRefillRate start")

	r.Mutex.Lock()

	data, _, err := r.Conn.Get(entity.INTERVAL_NODE_PATH)
	if err != nil {
		r.Mutex.Unlock() // Release the lock before panicking
		panic(err)
	}

	newRate, err := strconv.Atoi(string(data))
	if err != nil {
		r.Mutex.Unlock() // Release the lock before panicking
		panic("Invalid data in INTERVAL_NODE: " + err.Error())
	}

	log.Println("updateRefillRate middle", newRate)

	r.RefillRate = newRate
	r.Mutex.Unlock() // Release the lock before sending to the channel

	log.Println("updateRefillRate important", newRate)
	// Notify any other processes that the ticker has changed.
	r.TickerChanged <- true

	log.Println("Updated RefillRate to:", newRate)
}

func (r *RateLimiterNode) setNewInterval(ctx context.Context, newInterval int) error {
	stas, err := r.Conn.Set(strings.Join([]string{entity.ZNODE_PATH, entity.INTERVAL_NODE}, ""), []byte(strconv.Itoa(newInterval)), -1)
	if err != nil {
		return err
	}

	log.Println("set new interval successfully", stas)
	return nil
}

func (r *RateLimiterNode) getCurrentInterval(ctx context.Context) (time.Duration, error) {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	data, stas, err := r.Conn.Get(strings.Join([]string{entity.ZNODE_PATH, entity.INTERVAL_NODE}, ""))
	if err != nil {
		return 0, err
	}

	// convert data to int
	interval, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, err
	}

	log.Println("set new interval successfully", stas)
	return time.Duration(interval) * (time.Second), nil

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

func getNodeID(id string) string {
	return fmt.Sprintf("%s/guid_%s", entity.ZNODE_PATH, id)
}
