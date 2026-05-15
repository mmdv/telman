# TODO: add support for custom filename, perhaps from ENV variable.
check-free:
	@grep ',free' seen.csv | cut -d, -f1

# alias for check-free
check-available: check-free

check-taken:
	@grep ',taken' seen.csv | cut -d, -f1

check-invalid:
	@grep ',invalid' seen.csv | cut -d, -f1