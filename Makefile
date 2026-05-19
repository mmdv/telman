# Include the .env file. The dash (-include) prevents errors if the file does not exist.
-include .env
export CACHE_FILE_PATH
# Default FILE to CACHE_FILE_PATH if not explicitly provided
FILE ?= $(CACHE_FILE_PATH)

build:
	@go build -o bin/github-username-checker ./cmd/github-username-checker/

clean:
	rm -rf bin/

_require-file:
	@[ -n "$(FILE)" ] || { echo "Pass filename via FILE=... e.g., make check-free FILE=seen.csv ---OR--- set the CACHE_FILE_PATH environment variable." >&2; exit 1; }

check-free: _require-file
	@awk -F, '$$2=="free" {print $$1}' "$(FILE)"

check-available: check-free

check-taken: _require-file
	@awk -F, '$$2=="taken" {print $$1}' "$(FILE)"

check-invalid: _require-file
	@awk -F, '$$2=="invalid" {print $$1}' "$(FILE)"
