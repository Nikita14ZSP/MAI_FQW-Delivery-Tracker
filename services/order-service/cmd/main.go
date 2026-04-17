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
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	deliveryv1 "github.com/mozgovojnikita/delivery-tracker/gen/delivery/v1"
	menuv1 "github.com/mozgovojnikita/delivery-tracker/gen/menu/v1"
	orderv1 "github.com/mozgovojnikita/delivery-tracker/gen/order/v1"
	"github.com/mozgovojnikita/delivery-tracker/pkg/config"
	"github.com/mozgovojnikita/delivery-tracker/pkg/grpclient"
	"github.com/mozgovojnikita/delivery-tracker/pkg/health"
	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	pkglogger "github.com/mozgovojnikita/delivery-tracker/pkg/logger"
	"github.com/mozgovojnikita/delivery-tracker/pkg/middleware"
	"github.com/mozgovojnikita/delivery-tracker/pkg/postgres"
	grpchandler "github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/handler/grpc"
	httphandler "github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/handler/http"
	orderkafka "github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/repository"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/service"
)

const serviceName = "order-service"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	logger := pkglogger.NewWithService(cfg.Env, serviceName)
	slog.SetDefault(logger)

	// JWT_SECRET — fail fast if empty
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET environment variable is required")
		os.Exit(1)
	}

	// Migrations
	if err := runMigrations(cfg.DatabaseURL); err != nil {
		slog.Error("migration failed", "err", err)
		os.Exit(1)
	}

	// Postgres pool
	pool, err := postgres.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("postgres connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Redis client
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("invalid REDIS_URL", "err", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	// Kafka producer
	var publisher pkgkafka.EventPublisher
	if cfg.KafkaBrokers != "" {
		publisher = pkgkafka.NewProducer(cfg.KafkaBrokers)
		defer publisher.Close()
		slog.Info("kafka producer initialized", "brokers", cfg.KafkaBrokers)
	} else {
		slog.Warn("KAFKA_BROKERS not set, event publishing disabled")
	}

	// Repositories
	userRepo := repository.NewUserRepository(pool)
	orderRepo := repository.NewOrderRepository(pool)
	menuRepo := repository.NewMenuRepository(pool)

	// Wire delivery-service gRPC client (BKND-04): singleton with retry + circuit breaker.
	// Uses service_started (not service_healthy) to avoid circular depends_on with delivery-service.
	deliveryServiceGRPC := os.Getenv("DELIVERY_SERVICE_GRPC")
	if deliveryServiceGRPC == "" {
		deliveryServiceGRPC = "delivery-service:9090"
	}
	deliveryConn, err := grpc.NewClient(
		deliveryServiceGRPC,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpclient.NewCircuitBreakerInterceptor("delivery-service"),
			grpclient.NewRetryInterceptor(),
		),
	)
	if err != nil {
		slog.Error("failed to create delivery-service gRPC client", "err", err)
		os.Exit(1)
	}
	defer deliveryConn.Close()
	deliveryClient := service.NewGRPCDeliveryClient(deliveryv1.NewDeliveryServiceClient(deliveryConn))

	// Services
	authSvc := service.NewAuthService(userRepo, redisClient, jwtSecret)
	orderSvc := service.NewOrderService(orderRepo, publisher).WithDeliveryClient(deliveryClient)
	menuSvc := service.NewMenuService(menuRepo)

	// HTTP router (auth endpoints + health + metrics)
	r := chi.NewRouter()
	r.Use(middleware.Metrics)
	r.Use(middleware.TraceID)
	r.Get("/health", health.Handler())
	r.Handle("/metrics", promhttp.Handler())
	r.Mount("/v1/auth", httphandler.NewAuthHandler(authSvc).Routes())

	httpSrv := &http.Server{Addr: ":" + cfg.Port, Handler: r}

	// gRPC server with Prometheus interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcprom.UnaryServerInterceptor),
		grpc.ChainStreamInterceptor(grpcprom.StreamServerInterceptor),
	)
	orderv1.RegisterOrderServiceServer(grpcServer, grpchandler.NewOrderHandler(orderSvc, userRepo))
	menuv1.RegisterMenuServiceServer(grpcServer, grpchandler.NewMenuHandler(menuSvc))
	grpcprom.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		slog.Error("gRPC listen failed", "err", err, "port", cfg.GRPCPort)
		os.Exit(1)
	}

	slog.Info("service started", "service", serviceName, "http_port", cfg.Port, "grpc_port", cfg.GRPCPort)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	// Delivery event consumers (read side of the delivery->order status sync).
	// Guarded by the SAME cfg.KafkaBrokers condition as the producer: readers
	// require brokers, and order-service must stay startable without Kafka.
	// Distinct consumer groups guarantee order-service does NOT steal
	// tracking-service's delivery.* messages — Kafka delivers to every group
	// independently.
	if cfg.KafkaBrokers != "" {
		assignedReader := pkgkafka.NewConsumer(cfg.KafkaBrokers, pkgkafka.TopicDeliveryAssigned, "order-service-delivery-assigned")
		defer assignedReader.Close()
		statusReader := pkgkafka.NewConsumer(cfg.KafkaBrokers, pkgkafka.TopicDeliveryStatus, "order-service-delivery-status")
		defer statusReader.Close()
		go pkgkafka.RunConsumer(ctx, assignedReader, orderkafka.HandleDeliveryAssignedMessage(orderSvc))
		go pkgkafka.RunConsumer(ctx, statusReader, orderkafka.HandleDeliveryStatusMessage(orderSvc))
		slog.Info("delivery event consumers started",
			"groups", "order-service-delivery-assigned,order-service-delivery-status")
	}

	<-ctx.Done()
	slog.Info("shutdown signal received", "service", serviceName)

	// Shutdown order: gRPC first (waits for in-flight RPCs), then HTTP
	grpcServer.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP graceful shutdown failed", "err", err)
	}
	slog.Info("service stopped", "service", serviceName)
}

func runMigrations(databaseURL string) error {
	sep := "&"
	if len(databaseURL) > 0 && databaseURL[len(databaseURL)-1] == '/' {
		sep = "?"
	}
	m, err := migrate.New("file://migrations", databaseURL+sep+"x-migrations-table=order_schema_migrations")
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
