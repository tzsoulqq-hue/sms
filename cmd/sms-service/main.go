package main

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	smsv1 "github.com/byte-v-forge/sms/gen/go/byte/v/forge/contracts/sms/v1"
	smsinternalv1 "github.com/byte-v-forge/sms/gen/go/byte/v/forge/sms/private/v1"
	grpcadapter "github.com/byte-v-forge/sms/internal/adapters/grpc"
	"github.com/byte-v-forge/sms/internal/app"
	"github.com/byte-v-forge/sms/internal/core"
	"google.golang.org/grpc"
)

type config struct {
	ListenAddr         string
	PGDSN              string
	HTTPTimeoutSeconds int
	ProviderHTTPProxy  string
}

func main() {
	cfg := loadConfig()
	ctx := context.Background()

	configStore, err := app.NewPostgresProviderConfigStore(ctx, cfg.PGDSN)
	if err != nil {
		log.Fatalf("initialize SMS config store: %v", err)
	}
	defer configStore.Close()

	activationStore, err := app.NewPostgresActivationStore(ctx, cfg.PGDSN)
	if err != nil {
		log.Fatalf("initialize SMS activation store: %v", err)
	}
	defer activationStore.Close()
	httpTimeout := time.Duration(cfg.HTTPTimeoutSeconds) * time.Second
	activationService := app.NewActivationService(
		activationStore,
		app.NewProviderConfigRouteResolver(configStore),
		[]core.Provider{
			app.NewConfiguredProvider("5sim", configStore, httpTimeout, cfg.ProviderHTTPProxy),
			app.NewConfiguredProvider("herosms", configStore, httpTimeout, cfg.ProviderHTTPProxy),
			app.NewConfiguredProvider("smsbower", configStore, httpTimeout, cfg.ProviderHTTPProxy),
		},
		app.SystemClock{},
		app.RandomIDGenerator{},
	)
	activationService.StartCancelScheduler(ctx, 30*time.Second)
	adminService := app.NewProviderAdminService(configStore, activationService, activationStore, httpTimeout, cfg.ProviderHTTPProxy)

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", cfg.ListenAddr, err)
	}

	server := grpc.NewServer()
	smsv1.RegisterSmsActivationServiceServer(server, grpcadapter.NewActivationServer(activationService))
	smsinternalv1.RegisterSmsProviderAdminServiceServer(server, grpcadapter.NewProviderAdminServer(adminService))
	log.Printf("sms-service listening on %s", cfg.ListenAddr)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("sms-service failed: %v", err)
	}
}

func loadConfig() config {
	cfg := config{
		ListenAddr:         envDefault("SMS_LISTEN_ADDR", ":50051"),
		PGDSN:              envDefault("SMS_PG_DSN", envDefault("PG_DSN", "")),
		HTTPTimeoutSeconds: envInt("SMS_HTTP_TIMEOUT_SECONDS", 20),
		ProviderHTTPProxy:  envDefault("SMS_PROVIDER_HTTP_PROXY", ""),
	}
	if cfg.HTTPTimeoutSeconds <= 0 {
		cfg.HTTPTimeoutSeconds = 20
	}
	return cfg
}

func envDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("%s must be an integer: %v", name, err)
	}
	return parsed
}
