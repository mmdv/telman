package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Token         string
	CacheFilePath string
	InputFile     string
}

// TODO: support multiple files as arguments.
// TODO: add support for proxy, and for n+ usernames require either a token or a proxy.
func Load() (*Config, error) {
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

	return &Config{
		Token:         token,
		CacheFilePath: cacheFilePath,
		InputFile:     inputFile,
	}, nil
}
