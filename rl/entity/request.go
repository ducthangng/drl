package entity

import "time"

type Request struct {
	// Request is the struct that will be passed to the limiter to check if it can process the request
	// As each limiter has its own algorithm, the appropriate fields should be filled on a per-limiter basis
	// For example, the Leaky Bucket limiter will only need the key and the limit, while the Token Bucket limiter will need the key, the limit, and the burst
	Key                 string // or ID
	Limit               int
	Burst               int
	OriginatedTimeStamp int64
}

type Interval struct {
	Key      string
	Interval time.Duration
	Rate     int
	Burst    int
}
