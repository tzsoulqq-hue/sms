package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	smsv1 "github.com/byte-v-forge/sms/gen/go/byte/v/forge/contracts/sms/v1"
	grpcadapter "github.com/byte-v-forge/sms/internal/adapters/grpc"
	"github.com/byte-v-forge/sms/internal/app"
	"github.com/byte-v-forge/sms/internal/core"
	"github.com/byte-v-forge/sms/internal/providers/herosms"
	"google.golang.org/grpc"
)

type config struct {
	ListenAddr string

	Provider string
	APIKey   string
	Endpoint string

	ApplicationKey     string
	CountryISO2        string
	CountryCallingCode string
	ProviderCountryID  string
	UpstreamServiceKey string
	MaxPriceDecimal    string
	MaxPriceCurrency   string

	HTTPProxy          string
	HTTPTimeoutSeconds int
}

func main() {
	cfg := loadConfig()

	provider, err := newProvider(cfg)
	if err != nil {
		log.Fatalf("initialize SMS provider: %v", err)
	}

	activationService := app.NewActivationService(
		app.NewMemoryActivationStore(),
		app.NewStaticRouteResolver([]core.Route{routeFromConfig(cfg)}),
		[]core.Provider{provider},
		app.SystemClock{},
		app.RandomIDGenerator{},
	)

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", cfg.ListenAddr, err)
	}

	server := grpc.NewServer()
	smsv1.RegisterSmsActivationServiceServer(server, grpcadapter.NewActivationServer(activationService))
	log.Printf("sms-service listening on %s provider=%s application=%s country=%s", cfg.ListenAddr, cfg.Provider, cfg.ApplicationKey, cfg.CountryISO2)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("sms-service failed: %v", err)
	}
}

func loadConfig() config {
	cfg := config{
		ListenAddr:         envDefault("SMS_LISTEN_ADDR", ":50051"),
		Provider:           envDefault("SMS_PROVIDER", "herosms"),
		APIKey:             requiredEnv("SMS_PROVIDER_API_KEY"),
		Endpoint:           envDefault("SMS_PROVIDER_ENDPOINT", herosms.DefaultEndpoint),
		ApplicationKey:     envDefault("SMS_APPLICATION_KEY", "gopay"),
		CountryISO2:        strings.ToUpper(envDefault("SMS_COUNTRY_ISO2", "ID")),
		CountryCallingCode: strings.TrimPrefix(envDefault("SMS_COUNTRY_CALLING_CODE", "62"), "+"),
		ProviderCountryID:  requiredEnv("SMS_PROVIDER_COUNTRY_ID"),
		UpstreamServiceKey: requiredEnv("SMS_UPSTREAM_SERVICE_KEY"),
		MaxPriceDecimal:    envDefault("SMS_MAX_PRICE_DECIMAL", ""),
		MaxPriceCurrency:   envDefault("SMS_MAX_PRICE_CURRENCY", ""),
		HTTPProxy:          envDefault("SMS_HTTP_PROXY", ""),
		HTTPTimeoutSeconds: envInt("SMS_HTTP_TIMEOUT_SECONDS", 20),
	}
	if cfg.HTTPTimeoutSeconds <= 0 {
		cfg.HTTPTimeoutSeconds = 20
	}
	return cfg
}

func newProvider(cfg config) (core.Provider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "herosms":
		return herosms.New(herosms.Config{
			Endpoint: cfg.Endpoint,
			APIKey:   cfg.APIKey,
		}, newHTTPClient(cfg))
	default:
		return nil, fmt.Errorf("unsupported SMS provider %q", cfg.Provider)
	}
}

func routeFromConfig(cfg config) core.Route {
	return core.Route{
		ProviderKey:        strings.ToLower(strings.TrimSpace(cfg.Provider)),
		ApplicationKey:     strings.TrimSpace(cfg.ApplicationKey),
		UpstreamServiceKey: strings.TrimSpace(cfg.UpstreamServiceKey),
		CountryISO2:        strings.TrimSpace(cfg.CountryISO2),
		CountryCallingCode: strings.TrimSpace(cfg.CountryCallingCode),
		ProviderCountryID:  strings.TrimSpace(cfg.ProviderCountryID),
		MaxPrice: core.Money{
			CurrencyCode:  strings.TrimSpace(cfg.MaxPriceCurrency),
			AmountDecimal: strings.TrimSpace(cfg.MaxPriceDecimal),
		},
	}
}

func newHTTPClient(cfg config) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if strings.TrimSpace(cfg.HTTPProxy) != "" {
		proxyURL, err := url.Parse(cfg.HTTPProxy)
		if err != nil {
			log.Fatalf("invalid SMS_HTTP_PROXY: %v", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{
		Transport: transport,
		Timeout:   time.Duration(cfg.HTTPTimeoutSeconds) * time.Second,
	}
}

func envDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func requiredEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		log.Fatalf("%s is required", name)
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
