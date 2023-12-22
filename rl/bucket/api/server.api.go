package bucket

import (
	"context"

	"github.com/ducthangng/drl/rl/bucket/pb"
)

func AssignNewMaster(ctx context.Context, req *pb.AssignMasterRequest) (*pb.AssignMasterResponse, error) {

	// if a node received this request, mean that it is has been assigned as the new master
	// this mean 2 things:
	// 1. this node is the new master, it should update its state and the database state to reflect that.
	// 2. this node should update the other nodes to reflect the new master.

	// success := UpdateMasterState(req.NewMasterID)

	// return &pb.AssignMasterResponse{Success: success}, nil

	return nil, nil
}
