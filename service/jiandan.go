package service

import (
	"encoding/base64"
	"fmt"
	"focalors-go/slogger"
	"log/slog"
	"path"
	"strings"
	"time"

	"resty.dev/v3"
)

var jiadanLogger = slogger.New("service.jiandan")

const (
	JiadanAPIURL = "https://i.jandan.net/?oxwlxojflwblxbsapi=jandan.get_pic_comments"
	JiadanCDN    = "https://img.toto.im/large"
)

// VisitedStore is an interface for checking/saving visited status
type VisitedStore interface {
	// Exists returns >0 if the key exists
	Exists(key string) (int64, error)
	// Set stores a value with TTL
	Set(key string, value any, ttl time.Duration) error
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

// JiadanService provides access to Jiandan API
type JiadanService struct {
	client *resty.Client
	store  VisitedStore
}

// NewJiadanService creates a new Jiandan service with optional store for deduplication
func NewJiadanService(store VisitedStore) *JiadanService {
	return &JiadanService{
		client: resty.New().SetRetryCount(3).SetRetryWaitTime(1 * time.Second),
		store:  store,
	}
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

// GetTopPosts fetches top N posts with valid images, optionally filtering by a checker function
// skipChecker returns true if the comment should be skipped (e.g., already visited)
func (s *JiadanService) GetTopPosts(count int, page int, skipChecker func(comment JiadanComment) bool) ([]JiadanComment, error) {
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

		// Check if should skip (e.g., deduplication)
		if skipChecker != nil && skipChecker(comment) {
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
		morePosts, err := s.GetTopPosts(count-len(posts), result.CurrentPage+1, skipChecker)
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

// CreateSkipChecker returns a skipChecker function for deduplication
// Returns nil if no store is configured
func (s *JiadanService) CreateSkipChecker(targetKey string, syncMode bool) func(JiadanComment) bool {
	if s.store == nil {
		return nil
	}
	return func(comment JiadanComment) bool {
		commentKey := targetKey + ":" + comment.CommentId
		exists, _ := s.store.Exists(commentKey)
		if exists > 0 {
			if syncMode {
				return true
			}
			jiadanLogger.Debug("Comment already visited", slog.String("id", commentKey))
			return true
		}
		return false
	}
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

// FetchNewImages fetches new (non-visited) jiadan images with deduplication
// Returns base64 encoded images. Uses the service's store for deduplication.
func (s *JiadanService) FetchNewImages(targetKey string, count int, syncMode bool) ([]string, error) {
	skipChecker := s.CreateSkipChecker(targetKey, syncMode)

	// Fetch posts using the shared service
	posts, err := s.GetTopPosts(count, 0, skipChecker)
	if err != nil {
		return nil, err
	}

	// Collect all image URLs and save visited status
	var urls []string
	for _, post := range posts {
		commentKey := targetKey + ":" + post.CommentId
		s.SaveVisited(commentKey, post)
		urls = append(urls, GetImageURLs(post)...)
	}

	if len(urls) == 0 {
		return nil, nil
	}

	// Download and encode images
	return s.DownloadImages(urls)
}

// SaveVisited saves a comment as visited with TTL of 15 days from comment date
func (s *JiadanService) SaveVisited(commentKey string, comment JiadanComment) {
	if s.store == nil {
		return
	}
	parsedTime, err := time.Parse("2006-01-02 15:04:05", comment.CommentDate)
	if err != nil {
		jiadanLogger.Warn("Failed to parse time", slog.String("time", comment.CommentDate), slog.Any("error", err))
		parsedTime = time.Now()
	}
	// set key with expired after 15 days of parsedTime
	s.store.Set(commentKey, strings.Join(comment.Pics, ","), time.Until(parsedTime.AddDate(0, 0, 15)))
}
