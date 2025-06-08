package main

import (
	"context"
	"flag"
	"fmt"
	"focalors-go/client"
	"focalors-go/config"
	"focalors-go/slogger"
	"focalors-go/wechat"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

// Version information (can be overridden at build time)
var (
	ProgramName = "focalors-go"
	Version     = "0.1.0"
	BuildDate   = "unknown"
	GitCommit   = "unknown"
)

var prefixRegex = regexp.MustCompile(`^[#*%]`)

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
		logger.Info("Debug mode is enabled",
			slog.Any("Config", cfg))
	}

	// yunzai
	yunzai := client.NewYunzai(&cfg.Yunzai)
	defer yunzai.Stop()
	yunzai.AddMessageHandler(func(msg client.Response) bool {

		return false
	})

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

	// wc
	wc := wechat.NewWechatClient(cfg, redisClient, ctx)
	err = wc.InitAccount()
	if err != nil {
		logger.Error("Failed to init wechat account", slog.Any("error", err))
		return
	}

	yunzai.Start(ctx)
	yunzai.AddMessageHandler(func(msg client.Response) bool {
		for _, content := range msg.Content {
			if content.Type == "text" {
				content := strings.Trim(content.Data.(string), " \n")
				if content == "" {
					continue
				}
				wc.SendTextMessage([]wechat.MessageItem{
					{
						ToUserName:  msg.TargetId,
						TextContent: content,
						MsgType:     1,
						// TODO
						// AtWxIDList: []string{},
					},
				})
			}
			if content.Type == "image" {
				wc.SendImageNewMessage([]wechat.MessageItem{
					{
						ToUserName:   msg.TargetId,
						ImageContent: strings.TrimPrefix(content.Data.(string), "base64://"),
						MsgType:      2,
					},
				})
			}
		}
		return false
	})
	// Define the regex pattern
	wc.Start(ctx, func(message wechat.WechatMessage) {
		if message.MsgType == wechat.TextMessage && prefixRegex.MatchString(message.Content) {
			userType := "group"
			if message.ChatType == wechat.ChatTypePrivate {
				userType = "direct"
			}
			sent := client.Request{
				BotSelfId: "focalors",
				MsgId:     fmt.Sprintf("%d", message.MsgId),
				UserId:    message.FromUserId,
				GroupId:   message.FromGroupId,
				UserPM:    0,
				UserType:  userType,
				Content: []client.MessageContent{
					{
						Type: "text",
						Data: message.Content,
					},
				},
			}
			logger.Debug("Sending message to yunzai", slog.Any("request", sent))
			yunzai.Send(sent)
		}
	})

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("Context cancelled, shutting down...")

	// Give components time to shut down gracefully
	time.Sleep(2 * time.Second)
	logger.Info("Shutdown complete")
}
