package main

import (
	"context"
	"flag"
	"fmt"
	"focalors-go/config"
	"focalors-go/middlewares"
	"focalors-go/slogger"
	"focalors-go/wechat"
	"focalors-go/yunzai"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/redis/go-redis/v9"
)

// Version information (can be overridden at build time)
var (
	ProgramName = "focalors-go"
	Version     = "0.1.0"
	BuildDate   = "unknown"
	GitCommit   = "unknown"
)

// BuildInfo returns the current build information
func BuildInfo() string {
	return fmt.Sprintf("Version: %s, Built: %s, Commit: %s", Version, BuildDate, GitCommit)
}

func printVersionInfo() {
	fmt.Printf("%s version %s\n", ProgramName, Version)
	fmt.Printf("Built with %s on %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Build date: %s\n", BuildDate)
	fmt.Printf("Git commit: %s\n", GitCommit)
}

var logger = slogger.New("main")

func main() {
	// Define command line flags
	var (
		showVersion bool
		configPath  string
	)

	// Set up command line flags
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information (shorthand)")
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.StringVar(&configPath, "c", "", "Path to configuration file (shorthand)")
	flag.Parse()

	// Check if version flag is provided
	if showVersion {
		printVersionInfo()
		return
	}

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	logger.Info("Configuration loaded successfully:")

	if cfg.App.Debug {
		slogger.SetLogLevel(slog.LevelDebug)
		logger.Info("Debug mode is enabled",
			slog.Any("Config", cfg))
	}

	// Create a cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start a goroutine to handle shutdown signals
	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal", slog.String("signal", sig.String()))
		logger.Info("Initiating graceful shutdown...")
		cancel() // Cancel the context to signal all components to stop
	}()
	// redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	y := yunzai.NewYunzai(ctx, cfg)
	w := wechat.NewWechat(ctx, cfg)
	middlewares.NewMiddlewares(ctx, cfg, w, y, redisClient).Init()
	select {
	case err := <-runServiceAsync(y, w):
		logger.Error("Service failed", slog.Any("error", err))
		cancel()
	case <-ctx.Done():
		logger.Info("Context cancelled, shutting down...")
		return
	}
}

type Service interface {
	Start() error
	Stop()
}

func runServiceAsync(services ...Service) <-chan error {
	errChan := make(chan error, len(services))
	for _, service := range services {
		go func(service Service) {
			defer service.Stop()
			if err := service.Start(); err != nil {
				errChan <- err
			}
		}(service)
	}
	return errChan
}
