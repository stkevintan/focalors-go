package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"focalors-go/client"
	"focalors-go/config"
	"focalors-go/slogger"
	"log"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
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
	defer yunzai.Stop()
	yunzai.AddMessageHandler(func(msg client.Response) bool {

		return false
	})
	context := context.Background()
	defer context.Done()

	yunzai.Start(context)

	// Read terminal input and send it to yunzai as admin account in a loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		if !scanner.Scan() {
			break // EOF or error
		}
		input := scanner.Text()
		if input == "exit" || input == "quit" {
			logger.Info("Exiting on user command")
			break
		}
		if strings.HasPrefix(input, "#") && len(input) > 1 {
			yunzai.Send(client.Request{
				BotSelfId: "focalors",
				MsgId:     uuid.New().String(),
				UserId:    cfg.Yunzai.Admin,
				UserPM:    0,
				UserType:  "direct",
				Content: []client.MessageContent{
					{
						Type: "text",
						Data: input,
					},
				},
			})
		}
	}
}
