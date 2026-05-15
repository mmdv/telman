# Default FILE to CACHE_FILE_PATH if not explicitly provided
FILE ?= $(CACHE_FILE_PATH)

ifeq ($(strip $(FILE)),)
$(error Pass filename via FILE=... e.g., make check-free FILE=seen.csv ---OR--- set the CACHE_FILE_PATH environment variable.)
endif

check-free:
	@awk -F, '$$2=="free" {print $$1}' "$(FILE)"

check-available: check-free

check-taken:
	@awk -F, '$$2=="taken" {print $$1}' "$(FILE)"

check-invalid:
	@awk -F, '$$2=="invalid" {print $$1}' "$(FILE)"