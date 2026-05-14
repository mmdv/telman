package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type result struct {
	valid bool
	taken bool
}

type app struct {
	logger  *slog.Logger
	client  *http.Client
	token   string
	results map[string]result
}

var (
	ErrRateLimitExceeded = errors.New("API rate limit exceeded")
	ErrUnauthorized      = errors.New("invalid or expired token")
)

const (
	// Maximum number of simultaneous requests to the GitHub API.
	// Used to limit the number of concurrent requests on the application level
	// (goroutines) and the transport level.
	MaxSimultaneousRequests = 10
)

// TODO: support multiple files as arguments.
// TODO: save the processed usernames to a file or sqlite db to avoid reprocessing.
// TODO: add support for proxy, and for n+ usernames require either a token or a proxy.
// TODO: add support for exporting results to a file.
func main() {
	// Personal Access Token (PAT) for GitHub API
	// This token is used to authenticate requests to the GitHub API.
	// You can generate a token here: https://github.com/settings/tokens
	// Unauthenticated requests are limited to 60 per hour.
	// Authenticated requests are limited to 5,000 per hour.
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "error: GITHUB_TOKEN is not set")
		os.Exit(1)
	}

	// Check if at least one positional argument was provided
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "error: file with usernames is required")
		os.Exit(1)
	}
	file := flag.Arg(0)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

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

	app := &app{
		logger:  logger,
		client:  client,
		token:   token,
		results: make(map[string]result),
	}

	err := app.processFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func (a *app) processFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(MaxSimultaneousRequests)

	for scanner.Scan() {
		count++
		username := strings.TrimSpace(scanner.Text())

		if username == "" {
			continue
		}

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

			mu.Lock()
			defer mu.Unlock()
			a.results[username] = res

			return nil
		})
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if count == 0 {
		return fmt.Errorf("no usernames found in file")
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

func (a *app) processUsername(ctx context.Context, username string) (result, error) {
	if !isValidGithubUsername(username) {
		return result{}, nil
	}

	taken, err := a.checkTaken(ctx, username)
	if err != nil {
		if !errors.Is(err, ErrRateLimitExceeded) &&
			!errors.Is(err, ErrUnauthorized) &&
			!errors.Is(err, context.Canceled) {
			a.logger.Error(
				"failed to check availability",
				slog.String("username", username),
				slog.Any("error", err),
			)
		}
		return result{}, err
	}

	return result{
		valid: true,
		taken: taken,
	}, nil
}

func (a *app) checkTaken(ctx context.Context, username string) (bool, error) {
	a.logger.Info("checking availability", slog.String("username", username))

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
