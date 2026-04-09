package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	grpcprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	deliveryv1 "github.com/mozgovojnikita/delivery-tracker/gen/delivery/v1"
	trackingv1 "github.com/mozgovojnikita/delivery-tracker/gen/tracking/v1"
	"github.com/mozgovojnikita/delivery-tracker/pkg/config"
	"github.com/mozgovojnikita/delivery-tracker/pkg/grpclient"
	"github.com/mozgovojnikita/delivery-tracker/pkg/health"
	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	pkglogger "github.com/mozgovojnikita/delivery-tracker/pkg/logger"
	"github.com/mozgovojnikita/delivery-tracker/pkg/middleware"
	"github.com/mozgovojnikita/delivery-tracker/pkg/postgres"
	pkgredis "github.com/mozgovojnikita/delivery-tracker/pkg/redis"
	grpchandler "github.com/mozgovojnikita/delivery-tracker/services/tracking-service/internal/handler/grpc"
	"github.com/mozgovojnikita/delivery-tracker/services/tracking-service/internal/handler/ws"
	kafkahandler "github.com/mozgovojnikita/delivery-tracker/services/tracking-service/internal/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/tracking-service/internal/repository"
	"github.com/mozgovojnikita/delivery-tracker/services/tracking-service/internal/service"
)

const serviceName = "tracking-service"

// DeliveryGRPCClient wraps deliveryv1.DeliveryServiceClient and implements service.DeliveryClient.
type DeliveryGRPCClient struct {
	client deliveryv1.DeliveryServiceClient
}

// NewDeliveryGRPCClient creates a new DeliveryGRPCClient from a gRPC connection.
func NewDeliveryGRPCClient(conn *grpc.ClientConn) *DeliveryGRPCClient {
	return &DeliveryGRPCClient{client: deliveryv1.NewDeliveryServiceClient(conn)}
}

// GetDeliveriesByCourier calls the delivery-service gRPC to retrieve deliveries for a courier (TRAK-05).
func (c *DeliveryGRPCClient) GetDeliveriesByCourier(ctx context.Context, courierID string) ([]*deliveryv1.Delivery, error) {
	resp, err := c.client.GetDeliveriesByCourier(ctx, &deliveryv1.GetDeliveriesByCourierRequest{CourierId: courierID})
	if err != nil {
		return nil, err
	}
	return resp.GetDeliveries(), nil
}

func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	// Setup logger
	logger := pkglogger.NewWithService(cfg.Env, serviceName)
	slog.SetDefault(logger)

	// Run migrations
	if err := runMigrations(cfg.DatabaseURL); err != nil {
		slog.Error("migration failed", "err", err)
		os.Exit(1)
	}

	// Connect Postgres
	pool, err := postgres.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("postgres connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Connect Redis
	rdb, err := pkgredis.NewClient(cfg.RedisURL)
	if err != nil {
		slog.Error("redis connect failed", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// Create WebSocket Hub backed by Redis Pub/Sub
	hub := ws.NewHub(rdb)

	// Create repository and service
	repo := repository.NewLocationRepository(pool, rdb)

	// gRPC client to delivery-service for TRAK-05 (GetDeliveriesByCourier) with circuit breaker + retry (RSLN-01, RSLN-02)
	deliveryServiceGRPC := getEnvOrDefault("DELIVERY_SERVICE_GRPC", "delivery-service:9090")
	deliveryConn, err := grpc.NewClient(deliveryServiceGRPC,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpclient.NewCircuitBreakerInterceptor("delivery-service"),
			grpclient.NewRetryInterceptor(),
		),
	)
	if err != nil {
		slog.Error("delivery gRPC client failed", "err", err)
		os.Exit(1)
	}
	defer deliveryConn.Close()

	deliveryClient := NewDeliveryGRPCClient(deliveryConn)
	svc := service.NewTrackingService(repo, hub, deliveryClient)

	// Setup HTTP router
	r := chi.NewRouter()
	r.Use(middleware.TraceID)
	r.Get("/health", health.Handler())
	r.Handle("/metrics", promhttp.Handler())
	// WebSocket endpoint — auth middleware on gateway validates ?token= query param
	r.Get("/ws/orders/{order_id}", ws.ServeWS(hub))

	httpSrv := &http.Server{Addr: ":" + cfg.Port, Handler: r}

	// Create gRPC server with Prometheus interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcprom.UnaryServerInterceptor),
		grpc.ChainStreamInterceptor(grpcprom.StreamServerInterceptor),
	)
	trackingv1.RegisterTrackingServiceServer(grpcServer, grpchandler.NewTrackingHandler(svc))
	grpcprom.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		slog.Error("gRPC listen failed", "err", err, "port", cfg.GRPCPort)
		os.Exit(1)
	}

	// Kafka consumers with unique per-instance group IDs (D-09).
	// LastOffset is critical — avoids stale event replay on restart (Research Pitfall 4).
	instanceID := uuid.New().String()
	groupPrefix := "tracking-" + instanceID

	ordersCreatedReader := newTrackingConsumer(cfg.KafkaBrokers, pkgkafka.TopicOrdersCreated, groupPrefix+"-orders-created")
	ordersUpdatedReader := newTrackingConsumer(cfg.KafkaBrokers, pkgkafka.TopicOrdersUpdated, groupPrefix+"-orders-updated")
	deliveryAssignedReader := newTrackingConsumer(cfg.KafkaBrokers, pkgkafka.TopicDeliveryAssigned, groupPrefix+"-delivery-assigned")
	deliveryStatusReader := newTrackingConsumer(cfg.KafkaBrokers, pkgkafka.TopicDeliveryStatus, groupPrefix+"-delivery-status")
	defer ordersCreatedReader.Close()
	defer ordersUpdatedReader.Close()
	defer deliveryAssignedReader.Close()
	defer deliveryStatusReader.Close()

	slog.Info("service started", "service", serviceName, "http_port", cfg.Port, "grpc_port", cfg.GRPCPort)

	// Graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start goroutines
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("gRPC server error", "err", err)
			stop()
		}
	}()

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server error", "err", err)
			stop()
		}
	}()

	go pkgkafka.RunConsumer(ctx, ordersCreatedReader, kafkahandler.HandleOrderCreatedMessage(svc))
	go pkgkafka.RunConsumer(ctx, ordersUpdatedReader, kafkahandler.HandleOrderUpdatedMessage(svc))
	go pkgkafka.RunConsumer(ctx, deliveryAssignedReader, kafkahandler.HandleDeliveryAssignedMessage(svc))
	go pkgkafka.RunConsumer(ctx, deliveryStatusReader, kafkahandler.HandleDeliveryStatusMessage(svc))

	<-ctx.Done()
	slog.Info("shutdown signal received", "service", serviceName)

	// Graceful shutdown: gRPC first, then HTTP
	grpcServer.GracefulStop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP graceful shutdown failed", "err", err)
	}
	slog.Info("service stopped", "service", serviceName)
}

// newTrackingConsumer creates a Kafka reader with LastOffset to avoid stale event replay on restart.
// Unlike NewConsumer (FirstOffset), tracking service only needs real-time events — no catch-up needed.
func newTrackingConsumer(brokers, topic, groupID string) *kafkago.Reader {
	return kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        strings.Split(brokers, ","),
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
		StartOffset:    kafkago.LastOffset, // Only new events — no stale replay on restart
	})
}

func runMigrations(databaseURL string) error {
	sep := "&"
	if len(databaseURL) > 0 && databaseURL[len(databaseURL)-1] == '/' {
		sep = "?"
	}
	m, err := migrate.New("file://migrations", databaseURL+sep+"x-migrations-table=tracking_schema_migrations")
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}
	slog.Info("migrations complete", "service", serviceName)
	return nil
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
