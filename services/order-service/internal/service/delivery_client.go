package service

import (
	"context"

	deliveryv1 "github.com/mozgovojnikita/delivery-tracker/gen/delivery/v1"
)

// DeliveryClient is the order-service-local interface over delivery-service gRPC.
// Defined here (not in gen/) so we can mock in tests without depending on grpc.ClientConn.
type DeliveryClient interface {
	GetDeliveryByOrderID(ctx context.Context, in *deliveryv1.GetDeliveryByOrderIDRequest) (*deliveryv1.GetDeliveryByOrderIDResponse, error)
}

// grpcDeliveryClient adapts the generated deliveryv1.DeliveryServiceClient to DeliveryClient.
// The generated client has variadic grpc.CallOption args; this adapter drops them.
type grpcDeliveryClient struct {
	client deliveryv1.DeliveryServiceClient
}

// NewGRPCDeliveryClient wraps a generated deliveryv1.DeliveryServiceClient as a DeliveryClient.
func NewGRPCDeliveryClient(c deliveryv1.DeliveryServiceClient) DeliveryClient {
	return &grpcDeliveryClient{client: c}
}

func (g *grpcDeliveryClient) GetDeliveryByOrderID(ctx context.Context, in *deliveryv1.GetDeliveryByOrderIDRequest) (*deliveryv1.GetDeliveryByOrderIDResponse, error) {
	return g.client.GetDeliveryByOrderID(ctx, in)
}
