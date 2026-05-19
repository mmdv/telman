package config

import (
	"flag"
	"fmt"
	"github-username-checker/cmd/internal/config/proxy"
	"net/url"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	Token         string
	CacheFilePath string
	InputFile     string
	Proxy         *url.URL
}

// TODO: support multiple files as arguments.
// TODO: for n+ usernames require either a token or a proxy.
// Load loads the configuration from the environment and flags.
// The hierarchy of the configuration sources is:
// flags > environment variables > .env file > defaults
func Load() (*Config, error) {
	// Load .env into the process environment.
	// Silently ignore a missing .env because it is optional.
	_ = godotenv.Load()

	token := flag.String("token", "", "GitHub Personal Access Token [$GITHUB_TOKEN]")
	cacheFilePath := flag.String("cache", "", "Path to the CSV cache file [$CACHE_FILE_PATH]")
	proxyURL := flag.String("proxy", "", "Proxy URL, e.g. http://host:port [$PROXY_URL]")
	proxyUser := flag.String("proxy-user", "", "Proxy username [$PROXY_USER]")
	proxyPass := flag.String("proxy-pass", "", "Proxy password [$PROXY_PASS]")

	flag.Parse()

	setFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	// Apply env fallbacks for unset flags
	applyEnv := func(ptr *string, flagName, envVar string) {
		if !setFlags[flagName] {
			if envVal, ok := os.LookupEnv(envVar); ok {
				*ptr = envVal
			}
		}
	}

	applyEnv(token, "token", "GITHUB_TOKEN")
	applyEnv(cacheFilePath, "cache", "CACHE_FILE_PATH")
	applyEnv(proxyURL, "proxy", "PROXY_URL")
	applyEnv(proxyUser, "proxy-user", "PROXY_USER")
	applyEnv(proxyPass, "proxy-pass", "PROXY_PASS")

	// Validate
	if *token == "" {
		return nil, fmt.Errorf(
			"GitHub token is required: set GITHUB_TOKEN in the environment or pass -token flag",
		)
	}

	if *cacheFilePath == "" {
		return nil, fmt.Errorf(
			"cache file path is required: set CACHE_FILE_PATH in the environment or pass -cache flag",
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

	var finalProxy *url.URL
	if *proxyURL == "" && (*proxyUser != "" || *proxyPass != "") {
		return nil, fmt.Errorf("proxy URL is required when setting proxy credentials")
	}
	if *proxyURL != "" {
		u, err := proxy.Validate(*proxyURL, *proxyUser, *proxyPass)
		if err != nil {
			return nil, err
		}
		if err := proxy.Probe(u); err != nil {
			return nil, fmt.Errorf("proxy probing failed: %w", err)
		}
		finalProxy = u
	}

	return &Config{
		Token:         *token,
		CacheFilePath: *cacheFilePath,
		InputFile:     inputFile,
		Proxy:         finalProxy,
	}, nil
}
