package service

import (
	"encoding/base64"
	"fmt"
	"focalors-go/db"
	"focalors-go/slogger"
	"log/slog"
	"path"
	"time"

	"resty.dev/v3"
)

var jiadanLogger = slogger.New("service.jiandan")

const (
	JiadanAPIURL = "https://i.jandan.net/?oxwlxojflwblxbsapi=jandan.get_pic_comments"
	JiadanCDN    = "https://img.toto.im/large"
)

// JiadanService provides access to Jiandan API
type JiadanService struct {
	client *resty.Client
	store  *db.JiandanStore
}

// NewJiadanService creates a new Jiandan service with a store for deduplication
func NewJiadanService(store *db.JiandanStore) *JiadanService {
	return &JiadanService{
		client: resty.New().SetRetryCount(3).SetRetryWaitTime(1 * time.Second),
		store:  store,
	}
}

// JiadanComment represents a comment/post from Jiandan
type JiadanComment struct {
	CommentId     string   `json:"comment_ID"`
	CommentAuthor string   `json:"comment_author"`
	CommentDate   string   `json:"comment_date"`
	VotePositive  string   `json:"vote_positive"`
	VoteNegative  string   `json:"vote_negative"`
	Pics          []string `json:"pics"`
}

// JiadanResponse represents the API response from Jiandan
type JiadanResponse struct {
	CurrentPage int             `json:"current_page"`
	PageCount   int             `json:"page_count"`
	Comments    []JiadanComment `json:"comments"`
}

// UseCDN converts original image URL to CDN URL
func UseCDN(url string) string {
	file := path.Base(url)
	return fmt.Sprintf("%s/%s", JiadanCDN, file)
}

// FetchPage fetches a single page of comments from Jiandan
func (s *JiadanService) FetchPage(page int) (*JiadanResponse, error) {
	url := JiadanAPIURL
	if page > 0 {
		url = fmt.Sprintf("%s&page=%d", url, page)
	}

	result := &JiadanResponse{}
	resp, err := s.client.R().SetResult(result).Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("unexpected status code: %s", resp.Status())
	}
	return result, nil
}

// GetTopPosts fetches top N posts with valid images, skipping already-visited ones for the given targetId.
func (s *JiadanService) GetTopPosts(targetId string, count int, page int) ([]JiadanComment, error) {
	result, err := s.FetchPage(page)
	if err != nil {
		return nil, err
	}

	var posts []JiadanComment
	for _, comment := range result.Comments {
		// Skip admin posts
		if comment.CommentAuthor == "sein" {
			continue
		}

		// Skip already-visited comments
		if s.store != nil && s.store.IsVisited(targetId, comment.CommentId) {
			continue
		}

		// Check for valid images
		hasValidPic := len(comment.Pics) > 0
		if !hasValidPic {
			continue
		}

		posts = append(posts, comment)
		if len(posts) >= count {
			break
		}
	}

	// Fetch more pages if needed
	if len(posts) < count && result.CurrentPage < result.PageCount {
		morePosts, err := s.GetTopPosts(targetId, count-len(posts), result.CurrentPage+1)
		if err != nil {
			return posts, err // Return what we have so far
		}
		posts = append(posts, morePosts...)
	}

	return posts, nil
}

// GetImageURLs extracts CDN image URLs from a post
func GetImageURLs(comment JiadanComment) []string {
	var urls []string
	for _, pic := range comment.Pics {
		urls = append(urls, UseCDN(pic))
	}
	return urls
}

// DownloadImages downloads images from URLs and returns base64 encoded content
func (s *JiadanService) DownloadImages(urls []string) ([]string, error) {
	var base64Images []string
	for _, url := range urls {
		jiadanLogger.Debug("Downloading image", slog.String("url", url))
		resp, err := s.client.R().Get(url)
		if err != nil {
			jiadanLogger.Error("Failed to download image", slog.String("url", url), slog.Any("error", err))
			continue
		}

		if !resp.IsSuccess() {
			jiadanLogger.Error("Failed to download image, non-success status",
				slog.String("url", url),
				slog.String("status", resp.Status()),
			)
			continue
		}

		base64Str := base64.StdEncoding.EncodeToString(resp.Bytes())
		base64Images = append(base64Images, base64Str)
	}
	return base64Images, nil
}

// JiadanResult represents a fetched post with its downloaded images.
type JiadanResult struct {
	CommentId     string
	CommentAuthor string
	CommentDate   string
	VotePositive  string
	VoteNegative  string
	Images        []string // base64 encoded
}

// FetchNewImages fetches new (non-visited) jiandan posts with deduplication.
// Returns posts with downloaded base64 images.
func (s *JiadanService) FetchNewImages(targetId string, count int) ([]JiadanResult, error) {
	posts, err := s.GetTopPosts(targetId, count, 0)
	if err != nil {
		return nil, err
	}

	var results []JiadanResult
	for _, post := range posts {
		if s.store != nil {
			commentDate := parseCommentDate(post.CommentDate)
			s.store.MarkVisited(targetId, post.CommentId, post.Pics, commentDate)
		}
		urls := GetImageURLs(post)
		images, err := s.DownloadImages(urls)
		if err != nil {
			jiadanLogger.Warn("Failed to download images for post", slog.String("postId", post.CommentId), slog.Any("error", err))
			continue
		}
		results = append(results, JiadanResult{
			CommentId:     post.CommentId,
			CommentAuthor: post.CommentAuthor,
			CommentDate:   post.CommentDate,
			VotePositive:  post.VotePositive,
			VoteNegative:  post.VoteNegative,
			Images:        images,
		})
	}

	return results, nil
}

func parseCommentDate(dateStr string) time.Time {
	parsedTime, err := time.Parse("2006-01-02 15:04:05", dateStr)
	if err != nil {
		jiadanLogger.Warn("Failed to parse time", slog.String("time", dateStr), slog.Any("error", err))
		return time.Now()
	}
	return parsedTime
}
