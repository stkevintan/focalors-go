package tooling

import (
	"context"
	"fmt"
	"focalors-go/service"
	"log/slog"

	"github.com/openai/openai-go"
)

// JiadanTool provides humor posts from Jiandan
type JiadanTool struct {
	jiadan *service.JiadanService
}

type jiadanArgs struct {
	Count int `json:"count"`
}

// NewJiadanTool creates a new jiadan tool with the given service
func NewJiadanTool(jiadan *service.JiadanService) *JiadanTool {
	return &JiadanTool{
		jiadan: jiadan,
	}
}

func (j *JiadanTool) Name() string {
	return "jiandan_top"
}

func (j *JiadanTool) Definition() openai.FunctionDefinitionParam {
	return openai.FunctionDefinitionParam{
		Name: j.Name(),
		Description: openai.String("Get top humor image posts from Jiandan (煎蛋), a Chinese humor website. " +
			"Returns post info with image URLs that can be shared with users."),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"count": map[string]interface{}{
					"type":        "integer",
					"description": "Number of posts to fetch (1-5), defaults to 1",
					"minimum":     1,
					"maximum":     5,
					"default":     1,
				},
			},
		},
	}
}

func (j *JiadanTool) Execute(ctx context.Context, argsJSON string) (*ToolResult, error) {
	args, err := ParseArgs[jiadanArgs](argsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse jiandan args: %w", err)
	}

	count := args.Count
	if count < 1 {
		count = 1
	}
	if count > 5 {
		count = 5
	}

	logger.Info("Fetching Jiandan posts", slog.Int("count", count))

	// Use deduplication with target from context
	target := GetTarget(ctx)
	if target == "" {
		target = "gpt"
	}
	images, err := j.jiadan.FetchNewImages(target, count)
	if err != nil {
		logger.Error("Failed to fetch Jiandan posts", slog.Any("error", err))
		return NewToolResult("Failed to fetch Jiandan posts"), nil
	}

	if len(images) == 0 {
		return NewToolResult("No posts found on Jiandan"), nil
	}

	// Build result with post metadata and images
	result := NewToolResult(fmt.Sprintf("Found %d posts from 煎蛋无聊图", len(images)))
	result.AddText(fmt.Sprintf("**煎蛋无聊图** (%d帖)", len(images)))
	for i, post := range images {
		result.AddText(fmt.Sprintf("**%d.** %s (%s) 👍%s 👎%s",
			i+1, post.CommentAuthor, post.CommentDate, post.VotePositive, post.VoteNegative))
		for _, img := range post.Images {
			result.AddImage(img, "煎蛋无聊图")
		}
	}

	return result, nil
}
