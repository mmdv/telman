package cache

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// Define fields for the cache.
type Status string

const (
	StatusTaken   Status = "taken"
	StatusFree    Status = "free"
	StatusInvalid Status = "invalid"

	columnUsername string = "username"
	columnStatus   string = "status"
)

var expectedHeaders = []string{columnUsername, columnStatus}

// Define the interface for the cache manager.
type Manager interface {
	Load() error
	Exists(username string) bool
	Save(username string, status Status) error
	Close() error
}

func New(fileType, path string) (Manager, error) {
	switch fileType {
	case "csv":
		return newCSVManager(path)
	case "jsonl":
		return nil, fmt.Errorf("jsonl is not supported... yet")
	default:
		return nil, fmt.Errorf("invalid file type: %q, valid file types are: csv, jsonl", fileType)
	}
}

// Define the CSV manager.
type csvManager struct {
	path    string
	storage map[string]Status
	mu      sync.RWMutex
	file    *os.File
}

func newCSVManager(path string) (*csvManager, error) {
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
		_, err := outFile.WriteString(strings.Join(expectedHeaders, ",") + "\n")
		if err != nil {
			return nil, fmt.Errorf("write header: %w", err)
		}
	}

	return &csvManager{
		path:    path,
		storage: make(map[string]Status),
		file:    outFile,
	}, nil
}

// Load loads the processed usernames from a CSV file into the `storage` map.
func (cm *csvManager) Load() error {
	file, err := os.Open(cm.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read the headers.
	headers, err := reader.Read()
	if err != nil {
		// File exists, but is empty - return nil.
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("read header: %w", err)
	}

	err = validateHeaders(headers)
	if err != nil {
		return err
	}

	for {
		rec, err := reader.Read()

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return fmt.Errorf("read csv: %w", err)
		}

		if len(rec) < len(expectedHeaders) {
			fmt.Printf("invalid row length: expected %d got %d\n", len(expectedHeaders), len(rec))
			continue
		}

		username := rec[0]
		status := rec[1]

		// Validate status against constants.
		valid := validateStatus(status)
		if !valid {
			fmt.Printf("invalid status: %q\n", status)
			continue
		}

		cm.storage[username] = Status(status)
	}

	return nil
}

func (cm *csvManager) Exists(username string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	_, ok := cm.storage[username]
	return ok
}

func (cm *csvManager) Save(username string, status Status) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.storage[username] = status

	row := fmt.Sprintf("%s,%s\n", username, status)
	_, err := cm.file.WriteString(row)
	return err
}

func (cm *csvManager) Close() error {
	if cm.file != nil {
		return cm.file.Close()
	}
	return nil
}

func validateHeaders(headers []string) error {
	if len(headers) != len(expectedHeaders) {
		return fmt.Errorf("invalid headers: expected %d columns, got %d", len(expectedHeaders), len(headers))
	}

	for i, eh := range expectedHeaders {
		if headers[i] != eh {
			return fmt.Errorf("invalid column: expected %d column to be %q, got %q", i+1, eh, headers[i])
		}
	}

	return nil
}

func validateStatus(status string) bool {
	switch Status(status) {
	case StatusFree, StatusInvalid, StatusTaken:
		return true
	default:
		return false
	}
}

/// Target usage:
/// cache.New("csv", "path/to/cache.csv") // or JSONL in future
///
