package main

import (
	"context"
	"flag"
	"fmt"
	"focalors-go/config"
	"focalors-go/contract"
	"focalors-go/db"
	"focalors-go/middlewares"
	"focalors-go/provider/lark"
	"focalors-go/provider/wechat"
	"focalors-go/slogger"
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

	// Create shared Redis instance
	redis := db.NewRedis(ctx, &cfg.App.Redis)
	defer redis.Close()

	c, err := newGenericClient(cfg, redis)

	if err != nil {
		logger.Error("Failed to create client", slog.Any("error", err))
		return
	}

	go c.Start(ctx)

	mctx := middlewares.NewMiddlewareContext(ctx, c, cfg, redis)
	defer mctx.Close()

	m := middlewares.NewRootMiddleware(mctx)

	m.AddMiddlewares(
		middlewares.NewLogMsgMiddleware,
		middlewares.NewAdminMiddleware,
		middlewares.NewAccessMiddleware,
		middlewares.NewAvatarMiddleware,
		middlewares.NewJiadanMiddleware,
		middlewares.NewYunzaiMiddleware,
		middlewares.NewOpenAIMiddleware,
	)

	if err := m.Start(); err != nil {
		logger.Error("Failed to start middleware", slog.Any("error", err))
		return
	}
	defer m.Stop()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	logger.Info("Received shutdown signal", slog.String("signal", sig.String()))
	cancel()
}

func newGenericClient(cfg *config.Config, redis *db.Redis) (contract.GenericClient, error) {
	switch cfg.App.Platform {
	case "lark":
		return lark.NewLarkClient(cfg, redis)
	case "wechat", "":
		return wechat.NewWechat(cfg)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", cfg.App.Platform)
	}
}
