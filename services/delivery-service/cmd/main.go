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
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	grpcprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	deliveryv1 "github.com/mozgovojnikita/delivery-tracker/gen/delivery/v1"
	orderv1 "github.com/mozgovojnikita/delivery-tracker/gen/order/v1"
	"github.com/mozgovojnikita/delivery-tracker/pkg/config"
	"github.com/mozgovojnikita/delivery-tracker/pkg/grpclient"
	"github.com/mozgovojnikita/delivery-tracker/pkg/health"
	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	pkglogger "github.com/mozgovojnikita/delivery-tracker/pkg/logger"
	"github.com/mozgovojnikita/delivery-tracker/pkg/middleware"
	"github.com/mozgovojnikita/delivery-tracker/pkg/postgres"
	grpchandler "github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/handler/grpc"
	kafkahandler "github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/repository"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/service"
)

const serviceName = "delivery-service"

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

	// Create Kafka producer
	producer := pkgkafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	// Wire order-service gRPC client (singleton with retry + circuit breaker).
	// Uses service_started (not service_healthy) to avoid circular depends_on with order-service.
	orderServiceGRPC := os.Getenv("ORDER_SERVICE_GRPC")
	if orderServiceGRPC == "" {
		orderServiceGRPC = "order-service:9090"
	}
	orderConn, err := grpc.NewClient(
		orderServiceGRPC,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpclient.NewCircuitBreakerInterceptor("order-service"),
			grpclient.NewRetryInterceptor(),
		),
	)
	if err != nil {
		slog.Error("failed to create order-service gRPC client", "err", err)
		os.Exit(1)
	}
	defer orderConn.Close()
	orderClient := service.NewGRPCOrderClient(orderv1.NewOrderServiceClient(orderConn))

	// Create repository and service
	repo := repository.NewDeliveryRepository(pool)
	ratingRepo := repository.NewRatingRepository(pool)
	// DELIVERY_AUTO_ASSIGN gates v1.0 DLVR-02 (auto-assign nearest available courier on orders.created).
	// Default OFF: v2.0 manual-courier-selection product — couriers accept orders explicitly via AcceptOrder.
	// Set to "true" to restore legacy auto-assignment.
	autoAssign := os.Getenv("DELIVERY_AUTO_ASSIGN") == "true"
	svc := service.NewDeliveryService(repo, producer, ratingRepo, orderClient, autoAssign)

	// Setup HTTP router
	r := chi.NewRouter()
	r.Use(middleware.TraceID)
	r.Get("/health", health.Handler())
	r.Handle("/metrics", promhttp.Handler())

	httpSrv := &http.Server{Addr: ":" + cfg.Port, Handler: r}

	// Create gRPC server with Prometheus interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcprom.UnaryServerInterceptor),
		grpc.ChainStreamInterceptor(grpcprom.StreamServerInterceptor),
	)
	deliveryv1.RegisterDeliveryServiceServer(grpcServer, grpchandler.NewDeliveryHandler(svc))
	grpcprom.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		slog.Error("gRPC listen failed", "err", err, "port", cfg.GRPCPort)
		os.Exit(1)
	}

	// Create Kafka consumers
	reader := pkgkafka.NewConsumer(cfg.KafkaBrokers, pkgkafka.TopicOrdersCreated, "delivery-service")
	defer reader.Close()
	updatedReader := pkgkafka.NewConsumer(cfg.KafkaBrokers, pkgkafka.TopicOrdersUpdated, "delivery-service-updates")
	defer updatedReader.Close()

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

	go pkgkafka.RunConsumer(ctx, reader, kafkahandler.HandleOrderCreatedMessage(svc))
	go pkgkafka.RunConsumer(ctx, updatedReader, kafkahandler.HandleOrderUpdatedMessage(svc))

	go retryPendingLoop(ctx, svc)

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

// retryPendingLoop runs a ticker every 30 seconds to retry pending deliveries (D-04).
func retryPendingLoop(ctx context.Context, svc *service.DeliveryService) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := svc.RetryPendingDeliveries(ctx); err != nil {
				slog.Error("retry pending deliveries failed", "err", err)
			}
		}
	}
}

func runMigrations(databaseURL string) error {
	sep := "&"
	if len(databaseURL) > 0 && databaseURL[len(databaseURL)-1] == '/' {
		sep = "?"
	}
	m, err := migrate.New("file://migrations", databaseURL+sep+"x-migrations-table=delivery_schema_migrations")
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
