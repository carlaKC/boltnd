// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package offersrpc

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

// OffersClient is the client API for Offers service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type OffersClient interface {
	SendOnionMessage(ctx context.Context, in *SendOnionMessageRequest, opts ...grpc.CallOption) (*SendOnionMessageResponse, error)
	DecodeOffer(ctx context.Context, in *DecodeOfferRequest, opts ...grpc.CallOption) (*DecodeOfferResponse, error)
	SubscribeOnionPayload(ctx context.Context, in *SubscribeOnionPayloadRequest, opts ...grpc.CallOption) (Offers_SubscribeOnionPayloadClient, error)
}

type offersClient struct {
	cc grpc.ClientConnInterface
}

func NewOffersClient(cc grpc.ClientConnInterface) OffersClient {
	return &offersClient{cc}
}

func (c *offersClient) SendOnionMessage(ctx context.Context, in *SendOnionMessageRequest, opts ...grpc.CallOption) (*SendOnionMessageResponse, error) {
	out := new(SendOnionMessageResponse)
	err := c.cc.Invoke(ctx, "/offersrpc.Offers/SendOnionMessage", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *offersClient) DecodeOffer(ctx context.Context, in *DecodeOfferRequest, opts ...grpc.CallOption) (*DecodeOfferResponse, error) {
	out := new(DecodeOfferResponse)
	err := c.cc.Invoke(ctx, "/offersrpc.Offers/DecodeOffer", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *offersClient) SubscribeOnionPayload(ctx context.Context, in *SubscribeOnionPayloadRequest, opts ...grpc.CallOption) (Offers_SubscribeOnionPayloadClient, error) {
	stream, err := c.cc.NewStream(ctx, &Offers_ServiceDesc.Streams[0], "/offersrpc.Offers/SubscribeOnionPayload", opts...)
	if err != nil {
		return nil, err
	}
	x := &offersSubscribeOnionPayloadClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Offers_SubscribeOnionPayloadClient interface {
	Recv() (*SubscribeOnionPayloadResponse, error)
	grpc.ClientStream
}

type offersSubscribeOnionPayloadClient struct {
	grpc.ClientStream
}

func (x *offersSubscribeOnionPayloadClient) Recv() (*SubscribeOnionPayloadResponse, error) {
	m := new(SubscribeOnionPayloadResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// OffersServer is the server API for Offers service.
// All implementations must embed UnimplementedOffersServer
// for forward compatibility
type OffersServer interface {
	SendOnionMessage(context.Context, *SendOnionMessageRequest) (*SendOnionMessageResponse, error)
	DecodeOffer(context.Context, *DecodeOfferRequest) (*DecodeOfferResponse, error)
	SubscribeOnionPayload(*SubscribeOnionPayloadRequest, Offers_SubscribeOnionPayloadServer) error
	mustEmbedUnimplementedOffersServer()
}

// UnimplementedOffersServer must be embedded to have forward compatible implementations.
type UnimplementedOffersServer struct {
}

func (UnimplementedOffersServer) SendOnionMessage(context.Context, *SendOnionMessageRequest) (*SendOnionMessageResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SendOnionMessage not implemented")
}
func (UnimplementedOffersServer) DecodeOffer(context.Context, *DecodeOfferRequest) (*DecodeOfferResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DecodeOffer not implemented")
}
func (UnimplementedOffersServer) SubscribeOnionPayload(*SubscribeOnionPayloadRequest, Offers_SubscribeOnionPayloadServer) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeOnionPayload not implemented")
}
func (UnimplementedOffersServer) mustEmbedUnimplementedOffersServer() {}

// UnsafeOffersServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to OffersServer will
// result in compilation errors.
type UnsafeOffersServer interface {
	mustEmbedUnimplementedOffersServer()
}

func RegisterOffersServer(s grpc.ServiceRegistrar, srv OffersServer) {
	s.RegisterService(&Offers_ServiceDesc, srv)
}

func _Offers_SendOnionMessage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SendOnionMessageRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OffersServer).SendOnionMessage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/offersrpc.Offers/SendOnionMessage",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OffersServer).SendOnionMessage(ctx, req.(*SendOnionMessageRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Offers_DecodeOffer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DecodeOfferRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OffersServer).DecodeOffer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/offersrpc.Offers/DecodeOffer",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OffersServer).DecodeOffer(ctx, req.(*DecodeOfferRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Offers_SubscribeOnionPayload_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(SubscribeOnionPayloadRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(OffersServer).SubscribeOnionPayload(m, &offersSubscribeOnionPayloadServer{stream})
}

type Offers_SubscribeOnionPayloadServer interface {
	Send(*SubscribeOnionPayloadResponse) error
	grpc.ServerStream
}

type offersSubscribeOnionPayloadServer struct {
	grpc.ServerStream
}

func (x *offersSubscribeOnionPayloadServer) Send(m *SubscribeOnionPayloadResponse) error {
	return x.ServerStream.SendMsg(m)
}

// Offers_ServiceDesc is the grpc.ServiceDesc for Offers service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Offers_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "offersrpc.Offers",
	HandlerType: (*OffersServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SendOnionMessage",
			Handler:    _Offers_SendOnionMessage_Handler,
		},
		{
			MethodName: "DecodeOffer",
			Handler:    _Offers_DecodeOffer_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SubscribeOnionPayload",
			Handler:       _Offers_SubscribeOnionPayload_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "offersrpc.proto",
}
