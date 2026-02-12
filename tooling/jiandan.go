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
	targetKey := "jiandan:" + target
	skipChecker := j.jiadan.CreateSkipChecker(targetKey, false)
	posts, err := j.jiadan.GetTopPosts(count, 0, skipChecker)
	if err != nil {
		logger.Error("Failed to fetch Jiandan posts", slog.Any("error", err))
		return NewToolResult("Failed to fetch Jiandan posts"), nil
	}

	if len(posts) == 0 {
		return NewToolResult("No posts found on Jiandan"), nil
	}

	// Build result with interleaved text and images
	result := NewToolResult(fmt.Sprintf("Found %d posts from 煎蛋无聊图", len(posts)))
	result.AddText(fmt.Sprintf("**煎蛋无聊图** (%d张)", len(posts)))

	for i, post := range posts {
		// Mark as visited
		j.jiadan.SaveVisited(targetKey+":"+post.CommentId, post)

		// Add post info
		result.AddText(fmt.Sprintf("**%d.** %s (%s) 👍%s 👎%s",
			i+1, post.CommentAuthor, post.CommentDate, post.VotePositive, post.VoteNegative))

		// Download and add images for this post
		urls := service.GetImageURLs(post)
		images, err := j.jiadan.DownloadImages(urls)
		if err != nil {
			logger.Warn("Failed to download images for post", slog.String("postId", post.CommentId), slog.Any("error", err))
			result.AddText("糟糕,图片下载失败 \\`(╥﹏╥)\\`  \n")
			for i, url := range urls {
				result.AddText(fmt.Sprintf("- ![原图链接%d](%s)\n", i+1, url))
			}
			continue
		}
		for _, img := range images {
			result.AddImage(img, "煎蛋无聊图")
		}
	}

	return result, nil
}
