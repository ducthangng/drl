// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v4.25.1
// source: proto/node.proto

package pb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// RateLimiterServiceClient is the client API for RateLimiterService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type RateLimiterServiceClient interface {
	// Method to assign a new master node
	AssignNewMaster(ctx context.Context, in *AssignMasterRequest, opts ...grpc.CallOption) (*AssignMasterResponse, error)
	PingMaster(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error)
}

type rateLimiterServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewRateLimiterServiceClient(cc grpc.ClientConnInterface) RateLimiterServiceClient {
	return &rateLimiterServiceClient{cc}
}

func (c *rateLimiterServiceClient) AssignNewMaster(ctx context.Context, in *AssignMasterRequest, opts ...grpc.CallOption) (*AssignMasterResponse, error) {
	out := new(AssignMasterResponse)
	err := c.cc.Invoke(ctx, "/bucket.RateLimiterService/AssignNewMaster", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *rateLimiterServiceClient) PingMaster(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error) {
	out := new(PingResponse)
	err := c.cc.Invoke(ctx, "/bucket.RateLimiterService/PingMaster", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RateLimiterServiceServer is the server API for RateLimiterService service.
// All implementations must embed UnimplementedRateLimiterServiceServer
// for forward compatibility
type RateLimiterServiceServer interface {
	// Method to assign a new master node
	AssignNewMaster(context.Context, *AssignMasterRequest) (*AssignMasterResponse, error)
	PingMaster(context.Context, *PingRequest) (*PingResponse, error)
	mustEmbedUnimplementedRateLimiterServiceServer()
}

// UnimplementedRateLimiterServiceServer must be embedded to have forward compatible implementations.
type UnimplementedRateLimiterServiceServer struct {
}

func (UnimplementedRateLimiterServiceServer) AssignNewMaster(context.Context, *AssignMasterRequest) (*AssignMasterResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AssignNewMaster not implemented")
}
func (UnimplementedRateLimiterServiceServer) PingMaster(context.Context, *PingRequest) (*PingResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PingMaster not implemented")
}
func (UnimplementedRateLimiterServiceServer) mustEmbedUnimplementedRateLimiterServiceServer() {}

// UnsafeRateLimiterServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to RateLimiterServiceServer will
// result in compilation errors.
type UnsafeRateLimiterServiceServer interface {
	mustEmbedUnimplementedRateLimiterServiceServer()
}

func RegisterRateLimiterServiceServer(s grpc.ServiceRegistrar, srv RateLimiterServiceServer) {
	s.RegisterService(&RateLimiterService_ServiceDesc, srv)
}

func _RateLimiterService_AssignNewMaster_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AssignMasterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RateLimiterServiceServer).AssignNewMaster(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/bucket.RateLimiterService/AssignNewMaster",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RateLimiterServiceServer).AssignNewMaster(ctx, req.(*AssignMasterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _RateLimiterService_PingMaster_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RateLimiterServiceServer).PingMaster(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/bucket.RateLimiterService/PingMaster",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RateLimiterServiceServer).PingMaster(ctx, req.(*PingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// RateLimiterService_ServiceDesc is the grpc.ServiceDesc for RateLimiterService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var RateLimiterService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "bucket.RateLimiterService",
	HandlerType: (*RateLimiterServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "AssignNewMaster",
			Handler:    _RateLimiterService_AssignNewMaster_Handler,
		},
		{
			MethodName: "PingMaster",
			Handler:    _RateLimiterService_PingMaster_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/node.proto",
}
