package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"strings"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	deliveryv1 "github.com/mozgovojnikita/delivery-tracker/gen/delivery/v1"
	menuv1 "github.com/mozgovojnikita/delivery-tracker/gen/menu/v1"
	notificationv1 "github.com/mozgovojnikita/delivery-tracker/gen/notification/v1"
	orderv1 "github.com/mozgovojnikita/delivery-tracker/gen/order/v1"
	trackingv1 "github.com/mozgovojnikita/delivery-tracker/gen/tracking/v1"
	"github.com/mozgovojnikita/delivery-tracker/pkg/config"
	"github.com/mozgovojnikita/delivery-tracker/pkg/grpclient"
	"github.com/mozgovojnikita/delivery-tracker/pkg/health"
	pkglogger "github.com/mozgovojnikita/delivery-tracker/pkg/logger"
	pkgmw "github.com/mozgovojnikita/delivery-tracker/pkg/middleware"
	authhandler "github.com/mozgovojnikita/delivery-tracker/services/api-gateway/internal/handler/http"
	gatewaymw "github.com/mozgovojnikita/delivery-tracker/services/api-gateway/internal/middleware"
	_ "github.com/mozgovojnikita/delivery-tracker/services/api-gateway/docs" // swagger docs
)

const serviceName = "api-gateway"

// @title Delivery Tracker API
// @version 1.0
// @description REST API for the Delivery Tracker microservice platform
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	logger := pkglogger.NewWithService(cfg.Env, serviceName)
	slog.SetDefault(logger)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET environment variable is required")
		os.Exit(1)
	}

	orderServiceGRPC := getEnvOrDefault("ORDER_SERVICE_GRPC", "order-service:9090")
	orderServiceHTTP := getEnvOrDefault("ORDER_SERVICE_HTTP", "http://order-service:8080")
	deliveryServiceGRPC := getEnvOrDefault("DELIVERY_SERVICE_GRPC", "delivery-service:9090")
	notificationServiceGRPC := getEnvOrDefault("NOTIFICATION_SERVICE_GRPC", "notification-service:9090")
	trackingServiceGRPC := getEnvOrDefault("TRACKING_SERVICE_GRPC", "tracking-service:9090")
	trackingServiceHTTP := getEnvOrDefault("TRACKING_SERVICE_HTTP", "http://tracking-service:8080")

	// Redis for rate limiting
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("invalid REDIS_URL", "err", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	// gRPC connections to backend services with circuit breaker + retry interceptors (RSLN-01, RSLN-02)
	orderConn, err := grpc.NewClient(orderServiceGRPC,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpclient.NewCircuitBreakerInterceptor("order-service"),
			grpclient.NewRetryInterceptor(),
		),
	)
	if err != nil {
		slog.Error("gRPC client to order-service failed", "err", err)
		os.Exit(1)
	}
	defer orderConn.Close()

	deliveryConn, err := grpc.NewClient(deliveryServiceGRPC,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpclient.NewCircuitBreakerInterceptor("delivery-service"),
			grpclient.NewRetryInterceptor(),
		),
	)
	if err != nil {
		slog.Error("gRPC client to delivery-service failed", "err", err)
		os.Exit(1)
	}
	defer deliveryConn.Close()

	notificationConn, err := grpc.NewClient(notificationServiceGRPC,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpclient.NewCircuitBreakerInterceptor("notification-service"),
			grpclient.NewRetryInterceptor(),
		),
	)
	if err != nil {
		slog.Error("gRPC client to notification-service failed", "err", err)
		os.Exit(1)
	}
	defer notificationConn.Close()

	trackingConn, err := grpc.NewClient(trackingServiceGRPC,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpclient.NewCircuitBreakerInterceptor("tracking-service"),
			grpclient.NewRetryInterceptor(),
		),
	)
	if err != nil {
		slog.Error("gRPC client to tracking-service failed", "err", err)
		os.Exit(1)
	}
	defer trackingConn.Close()

	// grpc-gateway mux — forwards X-User-Id and X-User-Role as gRPC metadata
	gwMux := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(func(key string) (string, bool) {
			switch key {
			case "X-User-Id", "X-User-Role":
				return key, true
			default:
				return runtime.DefaultHeaderMatcher(key)
			}
		}),
	)
	if err := orderv1.RegisterOrderServiceHandlerClient(
		context.Background(), gwMux, orderv1.NewOrderServiceClient(orderConn),
	); err != nil {
		slog.Error("register order grpc-gateway handler failed", "err", err)
		os.Exit(1)
	}
	// MenuService lives in the same order-service process — reuse orderConn.
	if err := menuv1.RegisterMenuServiceHandlerClient(
		context.Background(), gwMux, menuv1.NewMenuServiceClient(orderConn),
	); err != nil {
		slog.Error("register menu grpc-gateway handler failed", "err", err)
		os.Exit(1)
	}
	if err := deliveryv1.RegisterDeliveryServiceHandlerClient(
		context.Background(), gwMux, deliveryv1.NewDeliveryServiceClient(deliveryConn),
	); err != nil {
		slog.Error("register delivery grpc-gateway handler failed", "err", err)
		os.Exit(1)
	}
	if err := notificationv1.RegisterNotificationServiceHandlerClient(
		context.Background(), gwMux, notificationv1.NewNotificationServiceClient(notificationConn),
	); err != nil {
		slog.Error("register notification grpc-gateway handler failed", "err", err)
		os.Exit(1)
	}
	if err := trackingv1.RegisterTrackingServiceHandlerClient(
		context.Background(), gwMux, trackingv1.NewTrackingServiceClient(trackingConn),
	); err != nil {
		slog.Error("register tracking grpc-gateway handler failed", "err", err)
		os.Exit(1)
	}

	// Reverse proxy for auth endpoints (order-service HTTP)
	authTarget, _ := url.Parse(orderServiceHTTP)
	authReverseProxy := httputil.NewSingleHostReverseProxy(authTarget)
	authProxyHandler := authhandler.NewAuthProxy(authReverseProxy)

	// Chi router with middleware chain
	r := chi.NewRouter()
	r.Use(pkgmw.Metrics)
	r.Use(pkgmw.TraceID)
	r.Use(gatewaymw.RateLimit(redisClient, 100000, 100000))
	r.Use(gatewaymw.AuthMiddleware(jwtSecret, "/v1/auth/", "/health", "/swagger/", "/metrics"))

	// Health + Swagger + Metrics
	r.Get("/health", health.Handler())
	r.Handle("/metrics", promhttp.Handler())
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	// Auth routes — proxied to order-service HTTP
	// Uses r.Route (NOT r.Mount) to avoid double-strip path bug.
	// Each handler sets r.URL.Path explicitly before proxying.
	r.Route("/v1/auth", func(r chi.Router) {
		r.Post("/register", authProxyHandler.Register)
		r.Post("/login", authProxyHandler.Login)
		r.Post("/refresh", authProxyHandler.RefreshToken)
})

	// WebSocket proxy to tracking-service (D-12).
	// grpc-gateway cannot proxy WebSocket — use httputil.ReverseProxy.
	// Go 1.23+ httputil.ReverseProxy natively supports WebSocket upgrades:
	// it preserves Upgrade/Connection hop-by-hop headers when backend returns 101
	// via switchProtocolCopier in net/http/httputil/reverseproxy.go.
	trackingTarget, _ := url.Parse(trackingServiceHTTP)
	wsProxy := httputil.NewSingleHostReverseProxy(trackingTarget)
	originalDirector := wsProxy.Director
	wsProxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Safety: explicitly preserve Upgrade header for WebSocket handshake
		// (Go 1.23+ handles this natively, but this guards against future changes)
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Connection", "Upgrade")
	}
	r.Get("/ws/orders/{order_id}", func(w http.ResponseWriter, r *http.Request) {
		// Auth middleware already validated JWT from ?token= query param
		// Forward order_id as path to tracking-service
		orderID := chi.URLParam(r, "order_id")
		r.URL.Path = "/ws/orders/" + orderID
		r.URL.Host = trackingTarget.Host
		r.URL.Scheme = trackingTarget.Scheme
		r.Host = trackingTarget.Host
		wsProxy.ServeHTTP(w, r)
	})

	// D-06: GPS UpdateLocation rate limit — 1 request per 5 seconds per user.
	// This is a ROUTE-SPECIFIC rate limit, separate from global Phase 2 rate limits.
	// Keyed by X-User-Id header (set by auth middleware before this handler runs).
	gpsRateLimit := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// D-06: only rate-limit POST /v1/tracking/location (GPS updates)
			if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/v1/tracking/location") {
				next.ServeHTTP(w, r)
				return
			}
			userID := r.Header.Get("X-User-Id")
			if userID == "" {
				next.ServeHTTP(w, r)
				return
			}
			key := "ratelimit:gps:" + userID
			count, err := redisClient.Incr(r.Context(), key).Result()
			if err != nil {
				slog.Warn("GPS rate limit Redis error, allowing request", "err", err)
				next.ServeHTTP(w, r)
				return
			}
			if count == 1 {
				redisClient.Expire(r.Context(), key, 5*time.Second) // 5-second window per D-06
			}
			if count > 1 {
				w.Header().Set("Retry-After", "5")
				http.Error(w, `{"error":"GPS update rate limit exceeded, max 1 per 5 seconds (D-06)"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	// --- RBAC + grpc-gateway proxy (D-01, D-02) ---
	// PathRBAC middleware checks role by method+path prefix before forwarding to gwMux.
	// Admin role bypasses all checks. GPS rate limit applied as additional middleware.
	// Group (not Route) so chi does NOT strip the /v1 prefix — gwMux needs full paths.
	r.Group(func(sub chi.Router) {
		sub.Use(gatewaymw.PathRBAC(gatewaymw.RBACRules()))
		sub.Use(func(next http.Handler) http.Handler {
			return gpsRateLimit(next)
		})
		sub.Handle("/v1/*", gwMux)
	})

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}

	slog.Info("service started", "service", serviceName, "port", cfg.Port,
		"order_grpc", orderServiceGRPC, "order_http", orderServiceHTTP,
		"tracking_grpc", trackingServiceGRPC, "tracking_http", trackingServiceHTTP)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutdown signal received", "service", serviceName)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}
	slog.Info("service stopped", "service", serviceName)
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
