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

	notificationv1 "github.com/mozgovojnikita/delivery-tracker/gen/notification/v1"
	"github.com/mozgovojnikita/delivery-tracker/pkg/config"
	"github.com/mozgovojnikita/delivery-tracker/pkg/health"
	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	pkglogger "github.com/mozgovojnikita/delivery-tracker/pkg/logger"
	"github.com/mozgovojnikita/delivery-tracker/pkg/middleware"
	"github.com/mozgovojnikita/delivery-tracker/pkg/postgres"
	grpchandler "github.com/mozgovojnikita/delivery-tracker/services/notification-service/internal/handler/grpc"
	kafkahandler "github.com/mozgovojnikita/delivery-tracker/services/notification-service/internal/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/notification-service/internal/repository"
	"github.com/mozgovojnikita/delivery-tracker/services/notification-service/internal/service"
)

const serviceName = "notification-service"

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

	// Repository and service
	repo := repository.NewNotificationRepository(pool)
	svc := service.NewNotificationService(repo)

	// Setup HTTP router
	r := chi.NewRouter()
	r.Use(middleware.TraceID)
	r.Get("/health", health.Handler())
	r.Handle("/metrics", promhttp.Handler())

	httpSrv := &http.Server{Addr: ":" + cfg.Port, Handler: r}

	// gRPC server with Prometheus interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcprom.UnaryServerInterceptor),
		grpc.ChainStreamInterceptor(grpcprom.StreamServerInterceptor),
	)
	notificationv1.RegisterNotificationServiceServer(grpcServer, grpchandler.NewNotificationHandler(svc))
	grpcprom.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		slog.Error("gRPC listen failed", "err", err, "port", cfg.GRPCPort)
		os.Exit(1)
	}

	// Kafka consumers: notification-service consumes 2 topics (delivery.assigned + orders.updated)
	deliveryReader := pkgkafka.NewConsumer(cfg.KafkaBrokers, pkgkafka.TopicDeliveryAssigned, "notification-service")
	defer deliveryReader.Close()

	ordersReader := pkgkafka.NewConsumer(cfg.KafkaBrokers, pkgkafka.TopicOrdersUpdated, "notification-service")
	defer ordersReader.Close()

	slog.Info("service started", "service", serviceName, "http_port", cfg.Port, "grpc_port", cfg.GRPCPort)

	// Graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start all 4 goroutines: gRPC, HTTP, 2 Kafka consumers
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
	go pkgkafka.RunConsumer(ctx, deliveryReader, kafkahandler.HandleDeliveryAssignedMessage(svc))
	go pkgkafka.RunConsumer(ctx, ordersReader, kafkahandler.HandleOrderUpdatedMessage(svc))

	<-ctx.Done()
	slog.Info("shutdown signal received", "service", serviceName)

	// Graceful shutdown: gRPC first (waits for in-flight RPCs), then HTTP
	grpcServer.GracefulStop()
	if err := deliveryReader.Close(); err != nil {
		slog.Error("failed to close delivery reader", "err", err)
	}
	if err := ordersReader.Close(); err != nil {
		slog.Error("failed to close orders reader", "err", err)
	}
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
	m, err := migrate.New("file://migrations", databaseURL+sep+"x-migrations-table=notification_schema_migrations")
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
