# telman

A command-line tool that checks a list of GitHub usernames for availability using the GitHub REST API. It reads usernames from one or more plain-text input files, checks each concurrently (up to 10 simultaneous requests), and records results to a CSV cache file so already-checked usernames are skipped on subsequent runs.

## How it works

Each username is queried via `HEAD /users/:username` and categorised as:

| Status | Meaning |
|---|---|
| `free` | No account found (404) |
| `taken` | Account exists (200) |
| `invalid` | Does not meet GitHub's username rules (alphanumeric + hyphens, no leading/trailing/consecutive hyphens, max 39 chars) |

## Configuration

Configuration is resolved in this priority order: **CLI flags > environment variables > `.env` file > defaults**.

| Flag | Env var | Description |
|---|---|---|
| `-token` | `GITHUB_TOKEN` | GitHub Personal Access Token (required) |
| `-cache` | `CACHE_FILE_PATH` | Path to the CSV cache file (required, must end in `.csv`) |
| `-proxy` | `PROXY_URL` | Optional proxy URL, e.g. `http://host:port` |
| `-proxy-user` | `PROXY_USER` | Optional proxy username |
| `-proxy-pass` | `PROXY_PASS` | Optional proxy password |

Input files are passed as positional arguments. Accepted extensions: `.txt`, `.lst`, `.list`, or no extension.

See `.env.example` for a template.

## Build

```bash
go build -o bin/telman ./cmd/telman/
```

Or using Make:

```bash
make build
```

## Usage

```bash
./bin/telman -token ghp_xxx -cache seen.csv candidates.txt
```

With a `.env` file containing `GITHUB_TOKEN` and `CACHE_FILE_PATH`, flags can be omitted:

```bash
./bin/telman candidates.txt
```

Multiple input files are supported:

```bash
./bin/telman list1.txt list2.txt list3.txt
```

## Cache file format

The cache is a CSV with two columns. It is created automatically if it does not exist, and results are appended on each run. Duplicate usernames across or within input files are handled safely.

```
username,status
alice,taken
xqzj,free
--bad,invalid
```

## Filtering results

```bash
make check-free FILE=seen.csv      # list all usernames with status=free
make check-taken FILE=seen.csv     # list all usernames with status=taken
make check-invalid FILE=seen.csv   # list all usernames with status=invalid
```

If `CACHE_FILE_PATH` is set in your environment or `.env`, `FILE` defaults to it and can be omitted.

## Generating candidates with crunch

[`crunch`](https://sourceforge.net/projects/crunch-wordlist/) is a wordlist generator. Install it via your package manager (e.g. `apt install crunch` or `brew install crunch`).

**Generate all permutations of specific letters (each used exactly once):**

```bash
crunch 4 4 -p rslmv
```

The first `4` is the minimum word length, the second `4` is the maximum. `-p rslmv` generates every permutation of the given letters, using each exactly once per word.

**Write output to a file:**

```bash
crunch 4 4 -p rslmv -o candidates.txt
```

**Generate combinations with non-repeating characters (charset mode):**

```bash
crunch 4 4 rslmv | grep -v '\(.\).*\1' > candidates.txt
```

Here `rslmv` is the character set crunch draws from (repeats allowed by default). The `grep` filter removes any word containing a repeated character, leaving only strings where every character is unique.

## Important caveat

A `free` result means no active public account was found at that username, **but it does not guarantee the username is available for registration**. GitHub retains usernames tied to suspended or soft-deleted accounts indefinitely and does not release them automatically — those accounts still return 404 and appear free to this tool. Treat the output as a filtered shortlist; the only definitive way to confirm a username is truly claimable is to attempt registration directly on GitHub.

## Notes

- A GitHub Personal Access Token is required to avoid aggressive unauthenticated rate limits. A classic token with no scopes (public read-only access) is sufficient.
- Results are appended to the cache file on each run. If the cache file does not exist, it is created automatically with the correct header.
- The tool safely handles duplicate usernames across or within input files.

## TODO

- [ ] Support JSONL as an alternative cache format.
- [ ] Support reading usernames from stdin, enabling direct piping (e.g. `crunch 4 4 -p rslmv | ./bin/telman`).
