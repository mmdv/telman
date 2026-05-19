package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	Token         string
	CacheFilePath string
	InputFile     string
}

// TODO: support multiple files as arguments.
// TODO: add support for proxy, and for n+ usernames require either a token or a proxy.
// Load loads the configuration from the environment and flags.
// The hierarchy of the configuration sources is:
// flags > environment variables > .env file > defaults
func Load() (*Config, error) {
	// Load .env into the process environment.
	// Silently ignore a missing .env because it is optional.
	_ = godotenv.Load()

	token := flag.String(
		"token",
		os.Getenv("GITHUB_TOKEN"),
		"GitHub Personal Access Token [$GITHUB_TOKEN]",
	)

	cacheFilePath := flag.String(
		"cache",
		os.Getenv("CACHE_FILE_PATH"),
		"Path to the CSV cache file [$CACHE_FILE_PATH]",
	)

	flag.Parse()

	if *token == "" {
		return nil, fmt.Errorf(
			"GitHub token is required: set GITHUB_TOKEN in the environment or pass -token",
		)
	}

	if *cacheFilePath == "" {
		return nil, fmt.Errorf(
			"cache file path is required: set CACHE_FILE_PATH in the environment or pass -cache",
		)
	}

	if filepath.Ext(*cacheFilePath) != ".csv" {
		return nil, fmt.Errorf("cache file must have a .csv extension")
	}

	inputFile := flag.Arg(0)
	if inputFile == "" {
		return nil, fmt.Errorf("usage: github-username-checker [flags] <input-file>")
	}

	ext := filepath.Ext(inputFile)
	allowed := map[string]bool{
		"":      true,
		".txt":  true,
		".lst":  true,
		".list": true,
	}
	if !allowed[ext] {
		return nil, fmt.Errorf("input file must have a .txt, .lst, .list, or no extension")
	}

	return &Config{
		Token:         *token,
		CacheFilePath: *cacheFilePath,
		InputFile:     inputFile,
	}, nil
}
