package main

import (
	"context"
	"flag"
	"fmt"
	"focalors-go/config"
	"focalors-go/db"
	"focalors-go/middlewares"
	"focalors-go/scheduler"
	"focalors-go/slogger"
	"focalors-go/wechat"
	"focalors-go/yunzai"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
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
	pwd, _ := os.Getwd()
	logger.Info("current working dir:", slog.String("pwd", pwd))
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	logger.Info("Configuration loaded successfully:")

	if cfg.App.Debug {
		slogger.SetLogLevel(slog.LevelDebug)
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

	w := wechat.NewWechat(ctx, cfg)
	w.Init()
	y := yunzai.NewYunzai(ctx, cfg)
	redis := db.NewRedis(ctx, &cfg.App.Redis)
	defer redis.Close()
	cron := scheduler.NewCronTask(redis)
	cron.Start()
	defer cron.Stop()

	m := middlewares.NewRootMiddleware(w, redis, cron, cfg)
	m.AddMiddlewares(
		middlewares.NewLogMsgMiddleware,
		middlewares.NewAdminMiddleware,
		middlewares.NewAccessMiddleware,
		middlewares.NewJiadanMiddleware,
		middlewares.NewOpenAIMiddleware,
		middlewares.NewBridgeMiddlewareFactory(y),
	)

	if err := m.Start(); err != nil {
		logger.Error("Failed to start middleware", slog.Any("error", err))
		return
	}
	defer m.Stop()

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
	// Run must be blocked until the service is stopped
	Run() error
	// Dispose is called when the service is stopped
	Dispose()
}

func runServiceAsync(services ...Service) <-chan error {
	errChan := make(chan error, len(services))
	for _, service := range services {
		go func(service Service) {
			defer service.Dispose()
			if err := service.Run(); err != nil {
				errChan <- err
			}
		}(service)
	}
	return errChan
}
