package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	ec2api "github.com/aws/aws-sdk-go-v2/service/ec2"
	ecsapi "github.com/aws/aws-sdk-go-v2/service/ecs"
	elbv2api "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	rdsapi "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/gocools-LLC/arch.gocools/internal/apiserver"
	internalaws "github.com/gocools-LLC/arch.gocools/internal/aws"
	awsdiscovery "github.com/gocools-LLC/arch.gocools/internal/discovery/aws"
	"github.com/gocools-LLC/arch.gocools/internal/graph"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("arch exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	addr := envOrDefault("ARCH_HTTP_ADDR", ":8081")
	runtimeConfig := internalaws.RuntimeConfigFromEnv()
	discoveryMode := runtimeConfig.NormalizedDiscoveryMode()

	logger.Info(
		"aws_discovery_configuration",
		"mode", discoveryMode,
		"region", runtimeConfig.Session.Region,
		"role_arn_set", runtimeConfig.Session.RoleARN != "",
		"session_name", runtimeConfig.Session.SessionName,
		"external_id_set", runtimeConfig.Session.ExternalID != "",
		"validate_on_start", runtimeConfig.ValidateOnStart,
	)

	graphService, err := buildGraphService(context.Background(), runtimeConfig)
	if err != nil {
		return err
	}

	srv := apiserver.New(apiserver.Config{
		Addr:         addr,
		Version:      version,
		Logger:       logger,
		GraphService: graphService,
	})

	serverErrCh := make(chan error, 1)
	go func() {
		logger.Info("starting arch service", "addr", addr, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrCh:
		return err
	case <-sigCtx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	logger.Info("arch service stopped")
	return nil
}

func buildGraphService(ctx context.Context, cfg internalaws.RuntimeConfig) (apiserver.GraphQueryService, error) {
	switch cfg.NormalizedDiscoveryMode() {
	case internalaws.DiscoveryModeStatic:
		return nil, nil
	case internalaws.DiscoveryModeAWS:
		if strings.TrimSpace(cfg.Session.Region) == "" {
			return nil, fmt.Errorf("arch aws discovery requires ARCH_AWS_REGION or AWS_REGION")
		}
		if cfg.ValidateOnStart {
			if err := internalaws.ValidateCredentials(ctx, cfg.Session, awsconfig.WithRegion(cfg.Session.Region)); err != nil {
				return nil, err
			}
		}

		awsCfg, err := internalaws.LoadConfig(ctx, cfg.Session, awsconfig.WithRegion(cfg.Session.Region))
		if err != nil {
			return nil, fmt.Errorf("load aws discovery config: %w", err)
		}

		discoverer := awsdiscovery.NewDiscoverer(awsdiscovery.Clients{
			EC2:   ec2api.NewFromConfig(awsCfg),
			ECS:   ecsapi.NewFromConfig(awsCfg),
			ELBV2: elbv2api.NewFromConfig(awsCfg),
			RDS:   rdsapi.NewFromConfig(awsCfg),
		}, awsdiscovery.Config{
			Region: cfg.Session.Region,
		})
		return graph.NewService(awsdiscovery.NewProvider(discoverer)), nil
	default:
		return nil, fmt.Errorf("unsupported ARCH_DISCOVERY_MODE: %q", cfg.DiscoveryMode)
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
