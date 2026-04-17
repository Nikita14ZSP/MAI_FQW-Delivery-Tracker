package service

import (
	"context"

	orderv1 "github.com/mozgovojnikita/delivery-tracker/gen/order/v1"
)

// OrderClient is the delivery-service-local interface over order-service gRPC.
// Defined here (not in gen/) so we can mock in tests without grpc.ClientConn.
// The signature omits the variadic grpc.CallOption to keep the interface simple.
type OrderClient interface {
	GetOrdersByIDs(ctx context.Context, in *orderv1.GetOrdersByIDsRequest) (*orderv1.GetOrdersByIDsResponse, error)
	// UpdateOrderStatus notifies the order-service of a status change (D-08 side effect).
	// Called after AcceptByCourier succeeds to transition the order to "confirmed".
	UpdateOrderStatus(ctx context.Context, in *orderv1.UpdateOrderStatusRequest) (*orderv1.UpdateOrderStatusResponse, error)
	// GetUsersByIDs fetches courier name info from order-service users table (RATE-05).
	GetUsersByIDs(ctx context.Context, in *orderv1.GetUsersByIDsRequest) (*orderv1.GetUsersByIDsResponse, error)
}

// grpcOrderClient adapts the generated orderv1.OrderServiceClient to OrderClient.
// The generated client has variadic grpc.CallOption args; this adapter drops them.
type grpcOrderClient struct {
	client orderv1.OrderServiceClient
}

// NewGRPCOrderClient wraps a generated orderv1.OrderServiceClient as an OrderClient.
func NewGRPCOrderClient(c orderv1.OrderServiceClient) OrderClient {
	return &grpcOrderClient{client: c}
}

func (g *grpcOrderClient) GetOrdersByIDs(ctx context.Context, in *orderv1.GetOrdersByIDsRequest) (*orderv1.GetOrdersByIDsResponse, error) {
	return g.client.GetOrdersByIDs(ctx, in)
}

func (g *grpcOrderClient) UpdateOrderStatus(ctx context.Context, in *orderv1.UpdateOrderStatusRequest) (*orderv1.UpdateOrderStatusResponse, error) {
	return g.client.UpdateOrderStatus(ctx, in)
}

func (g *grpcOrderClient) GetUsersByIDs(ctx context.Context, in *orderv1.GetUsersByIDsRequest) (*orderv1.GetUsersByIDsResponse, error) {
	return g.client.GetUsersByIDs(ctx, in)
}
