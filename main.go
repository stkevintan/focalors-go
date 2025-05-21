package main

import (
	"context"
	"flag"
	"fmt"
	"focalors-go/client"
	"focalors-go/config"
	"focalors-go/slogger"
	"log"
	"log/slog"
	"runtime"
	"time"
)

// Version information
const (
	ProgramName = "focalors-go"
	Version     = "0.1.0"
)

// BuildInfo returns the current build information
func BuildInfo() string {
	return time.Now().Format(time.RFC3339)
}

func printVersionInfo() {
	fmt.Printf("%s version %s\n", ProgramName, Version)
	fmt.Printf("Built with %s on %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Build timestamp: %s\n", BuildInfo())
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
		logger.Info("Debug mode is enabled",
			slog.Any("App", cfg.App),
			slog.Any("Yunzai", cfg.Yunzai))
	}

	yunzai := client.NewYunzai(&cfg.Yunzai)
	yunzai.AddMessageHandler(func(msg client.Message) bool {
		if msg.Action == "get_version" {
			yunzai.Send(client.BotVersionConstant)
			return true
		}

		if msg.Action == "get_status" {
			yunzai.Send(client.StatusUpdate{
				Good: true,
				Bots: client.BotStatusConstant,
			})
			return true
		}

		if msg.Action == "upload_file" {

		}

		if msg.Action == "get_self_info" {

		}

		if msg.Action == "get_friend_list" {
		}

		if msg.Action == "get_group_list" {
		}

		if msg.Action == "get_group_member_info" {

		}

		if msg.Action == "get_group_member_list" {
		}

		if msg.Action == "send_message" {

		}

		return false
	})
	context := context.Background()
	yunzai.Start(context)
	// Wait forever
	select {}
}
