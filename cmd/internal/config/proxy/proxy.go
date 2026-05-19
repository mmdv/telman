package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Validate parses and validates the proxy URL, username and password.
func Validate(rawURL, rawUser, rawPass string) (*url.URL, error) {
	proxyURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	switch proxyURL.Scheme {
	case "http", "https", "socks5":
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s, must be http, https or socks5", proxyURL.Scheme)
	}

	if rawPass != "" {
		if rawUser == "" {
			return nil, fmt.Errorf("proxy password is specified but proxy username is not")
		}
		proxyURL.User = url.UserPassword(rawUser, rawPass)
	}

	return proxyURL, nil
}

// Probe checks if the proxy is reachable by making a GET request to https://github.com.
func Probe(u *url.URL) error {
	const timeoutSeconds = 4

	fmt.Printf("probing proxy: %s, please wait\n", u.String())

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(u),
		},
		Timeout: timeoutSeconds * time.Second,
	}

	resp, err := client.Get("https://github.com")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("request timed out after %d seconds", timeoutSeconds)
		}
		return err
	}
	defer resp.Body.Close()

	return nil
}
