package cache_test

import (
	"fmt"
	"github-username-checker/cmd/internal/cache"
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tc := []struct {
		name     string
		fileType string
		path     string
		wantErr  bool
	}{
		{
			name:     "non existent csv file, but the parent dir also does not exist",
			fileType: "csv",
			path:     "no-exist/cache.csv",
			wantErr:  true,
		},
		{
			name:     "non existent csv file, existing parent dir (should create the file)",
			fileType: "csv",
			path:     filepath.Join(t.TempDir(), "cache.csv"),
			wantErr:  false,
		},
		{
			name:     "jsonl file (not implemented yet)",
			fileType: "jsonl",
			path:     "cache.jsonl",
			wantErr:  true,
		},
		{
			name:     "invalid file type",
			fileType: "tsv",
			path:     "cache.tsv",
			wantErr:  true,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cm, err := cache.New(tt.fileType, tt.path)

			if (err != nil) != tt.wantErr {
				t.Errorf("cache.New(%q, %q): expected error: %v, got: %v", tt.fileType, tt.path, tt.wantErr, err != nil)
				return
			}

			if tt.wantErr {
				return
			}

			if cm == nil {
				t.Error("expected cache manager to be not nil")
			} else {
				cm.Close()
			}
		})
	}
}

func TestLoad(t *testing.T) {
	t.Parallel()

	tc := []struct {
		name         string
		fileContents string
		wantErr      bool
		checkExists  map[string]bool
	}{
		{
			name:         "empty file",
			fileContents: "",
			wantErr:      false,
		},
		{
			name:         "valid headers",
			fileContents: "username,status\n",
			wantErr:      false,
		},
		{
			name:         "invalid headers",
			fileContents: "invalid,headers\n",
			wantErr:      true,
		},
		{
			name:         "extra headers",
			fileContents: "username,status,extra\n",
			wantErr:      true,
		},
		{
			name:         "corrupt csv file headers",
			fileContents: "username,,status\n",
			wantErr:      true,
		},
		{
			name:         "corrupt csv file rows",
			fileContents: "username,status\nfoo,,\"taken\n",
			wantErr:      true,
		},
		{
			name:         "invalid status row skipped",
			fileContents: "username,status\nbob,invalid-status\nalice,taken\n",
			wantErr:      false,
			checkExists:  map[string]bool{"bob": false, "alice": true},
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpFile := filepath.Join(t.TempDir(), "cache.csv")
			err := os.WriteFile(tmpFile, []byte(tt.fileContents), 0644)
			if err != nil {
				t.Fatalf("write file: %v", err)
			}

			cm, err := cache.New("csv", tmpFile)
			if err != nil {
				t.Fatal("cache manager init:", err)
			}
			defer cm.Close()

			err = cm.Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load(): error = %v, wantErr = %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkExists != nil {
				for u, ok := range tt.checkExists {
					if cm.Exists(u) != ok {
						t.Errorf("username %q exists: expected %v, got %v", u, ok, cm.Exists(u))
					}
				}
			}
		})
	}

	t.Run("no file", func(t *testing.T) {
		t.Parallel()

		tmpFile := filepath.Join(t.TempDir(), "cache.csv")

		cm, err := cache.New("csv", tmpFile)
		if err != nil {
			t.Fatal("cache manager init:", err)
		}
		defer cm.Close()

		if err = os.Remove(tmpFile); err != nil {
			t.Fatal("remove file:", err)
		}

		err = cm.Load()
		if err != nil {
			t.Fatal("expected nil, got error:", err)
		}
	})
}

func TestSaveAndExists(t *testing.T) {
	t.Parallel()

	tmpPath := filepath.Join(t.TempDir(), "cache.csv")
	cm, err := cache.New("csv", tmpPath)
	if err != nil {
		t.Fatal("init:", err)
	}
	defer cm.Close()

	err = cm.Load()
	if err != nil {
		t.Fatal("load:", err)
	}

	username := "torvalds"
	if err = cm.Save(username, cache.StatusTaken); err != nil {
		t.Fatal("save:", err)
	}

	if !cm.Exists(username) {
		t.Fatal("expected username to be in the cache, got false")
	}

	// Ensure the changes are persisted to the file.
	content, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatal("read file:", err)
	}

	want := "username,status\n" + fmt.Sprintf("%s,%s\n", username, cache.StatusTaken)
	if string(content) != want {
		t.Fatalf("expected file content to be %q, got %q", want, string(content))
	}
}
