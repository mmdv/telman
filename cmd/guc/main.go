package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"github-username-checker/cmd/internal"
	"github-username-checker/cmd/internal/cache"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type config struct {
	token         string
	cacheFilePath string
	inputFile     string
}

var (
	ErrRateLimitExceeded = errors.New("API rate limit exceeded")
	ErrUnauthorized      = errors.New("invalid or expired token")
)

type app struct {
	client *http.Client
	token  string
	cache  cache.Manager
}

const (
	// Maximum number of simultaneous requests to the GitHub API.
	// Used to limit the number of concurrent requests on the application level
	// (goroutines) and the transport level.
	MaxSimultaneousRequests = 10
)

// TODO: support multiple files as arguments.
// TODO: add support for proxy, and for n+ usernames require either a token or a proxy.
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: MaxSimultaneousRequests,
			MaxConnsPerHost:     MaxSimultaneousRequests,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	cacheManager, err := cache.New("csv", cfg.cacheFilePath)
	if err != nil {
		return fmt.Errorf("cache manager: init: %w", err)
	}

	app := &app{
		client: client,
		token:  cfg.token,
		cache:  cacheManager,
	}

	err = cacheManager.Load()
	if err != nil {
		return fmt.Errorf("cache manager: load cache: %w", err)
	}

	outFile, err := initOutputFile(cfg.cacheFilePath, []string{"username", "status"})
	if err != nil {
		return fmt.Errorf("init output file: %w", err)
	}
	defer outFile.Close()

	err = app.processFile(cfg.inputFile, outFile)
	if err != nil {
		return fmt.Errorf("process file: %w", err)
	}

	return nil
}

func loadConfig() (*config, error) {
	// Personal Access Token (PAT) for GitHub API
	// This token is used to authenticate requests to the GitHub API.
	// You can generate a token here: https://github.com/settings/tokens
	// Unauthenticated requests are limited to 60 per hour.
	// Authenticated requests are limited to 5,000 per hour.
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN is not set")
	}

	cacheFilePath := os.Getenv("CACHE_FILE_PATH")
	if cacheFilePath == "" {
		return nil, fmt.Errorf("CACHE_FILE_PATH is not set")
	}
	ext := filepath.Ext(cacheFilePath)
	if ext != ".csv" {
		return nil, fmt.Errorf("only CSV files are supported")
	}

	// Check if at least one positional argument was provided
	flag.Parse()
	if flag.NArg() < 1 {
		return nil, fmt.Errorf("file with usernames is required")
	}
	inputFile := flag.Arg(0)

	return &config{
		token:         token,
		cacheFilePath: cacheFilePath,
		inputFile:     inputFile,
	}, nil
}

func (a *app) processFile(path string, outFile *os.File) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(MaxSimultaneousRequests)

	// TODO: move the seen map to the app struct, so we dedupe across multiple files.
	seen := make(map[string]struct{})

	for scanner.Scan() {
		count++
		username := strings.TrimSpace(scanner.Text())

		if username == "" {
			continue
		}

		if a.cache.Exists(username) {
			fmt.Printf("%s: skipping, already checked\n", username)
			continue

		}

		if _, ok := seen[username]; ok {
			// This username is already checked or the check is in progress.
			// Ignore it silently to not spam the console.
			continue
		}
		seen[username] = struct{}{}

		if ctx.Err() != nil {
			break
		}

		g.Go(func() error {
			res, err := a.processUsername(ctx, username)
			// We only return an error from processUsername() if we can't process the
			// rest of the usernames, so we're aborting the entire file.
			// TODO: with multiple files, stop processing the other files as well.
			if err != nil {
				return fmt.Errorf("aborting on username %q: %w", username, err)
			}

			status := cache.StatusTaken
			if !res.Taken {
				status = cache.StatusFree
			}
			if !res.Valid {
				status = cache.StatusInvalid
			}

			row := fmt.Sprintf("%s,%s\n", username, status)

			mu.Lock()
			_, err = outFile.WriteString(row)
			if err != nil {
				mu.Unlock()
				return fmt.Errorf("write to output file: %w", err)
			}
			mu.Unlock()

			a.cache.Set(username, status)

			return nil
		})
	}

	if count == 0 {
		return fmt.Errorf("no usernames found in file")
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

func initOutputFile(path string, headers []string) (*os.File, error) {
	outFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open output file: %w", err)
	}

	// Write the CSV header if the file is empty.
	stat, err := outFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("check if output file is empty: %w", err)
	}
	if stat.Size() == 0 {
		_, err := outFile.WriteString(strings.Join(headers, ",") + "\n")
		if err != nil {
			return nil, fmt.Errorf("write header: %w", err)
		}
	}

	return outFile, nil
}

func (a *app) processUsername(ctx context.Context, username string) (internal.CheckResult, error) {
	if !isValidGithubUsername(username) {
		return internal.CheckResult{}, nil
	}

	taken, err := a.checkTaken(ctx, username)
	if err != nil {
		if !errors.Is(err, ErrRateLimitExceeded) &&
			!errors.Is(err, ErrUnauthorized) &&
			!errors.Is(err, context.Canceled) {
			fmt.Printf("check availability for username: %q, error: %v\n", username, err)
		}
		return internal.CheckResult{}, err
	}

	return internal.CheckResult{
		Valid: true,
		Taken: taken,
	}, nil
}

func (a *app) checkTaken(ctx context.Context, username string) (bool, error) {
	fmt.Printf("%s: checking availability...\n", username)

	url := fmt.Sprintf("https://api.github.com/users/%s", username)

	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.token))

	resp, err := a.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	case http.StatusForbidden:
		return false, ErrRateLimitExceeded
	case http.StatusUnauthorized:
		return false, ErrUnauthorized
	default:
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

// isValidGithubUsername checks if a string meets GitHub's username constraints.
func isValidGithubUsername(username string) bool {
	length := len(username)

	// GitHub usernames cannot be empty and have a maximum length of 39 characters.
	if length == 0 || length > 39 {
		return false
	}

	// Cannot start or end with a hyphen.
	if username[0] == '-' || username[length-1] == '-' {
		return false
	}

	// Cannot contain consecutive hyphens.
	if strings.Contains(username, "--") {
		return false
	}

	// Check for invalid characters.
	for i := range length {
		c := username[i]
		isLower := c >= 'a' && c <= 'z'
		isUpper := c >= 'A' && c <= 'Z' // Included in case of mixed-case input
		isDigit := c >= '0' && c <= '9'
		isHyphen := c == '-'

		if !isLower && !isUpper && !isDigit && !isHyphen {
			return false
		}
	}

	return true
}
