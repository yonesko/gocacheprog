# gocacheprog

A tool to cache Go build artifacts in Redis. Implements protocol https://pkg.go.dev/cmd/go/internal/cacheprog.

Used in production and accelerated builds in Gitlab CI/CD by 4 times.

## Installation

```bash
go install github.com/yonesko/gocacheprog@latest
```

## Usage

The tool works as a proxy between `go build` and the file system, intercepting read/write requests for build artifacts.
It uses local storage (a directory on disk) as the primary storage and Redis as an external cache for sharing artifacts
between different builds.

### Command Line Options

- `-dir` - required parameter specifying the local directory for cache storage
- `-r-urls` - comma-separated list of Redis server addresses
- `-r-usr` - Redis username (optional)
- `-r-pwd` - Redis password (optional)
- `-r-prefix` - string to prefix Redis cache keys (optional)
- `-log-metrics` - enable metrics logging (optional)
- `-log-req` - enable request logging  (optional)
- `-log-resp` - enable response logging  (optional)

### Example Usage

```yaml
GOCACHEPROG=gocacheprog -r-urls "localhost:6379" -dir "/tmp/cache"
```

## Architecture

The tool implements a three-tier data storage system:

1. Local file storage - primary storage for artifacts
2. Redis - external cache for sharing between builds
3. If redis become unavailable this tool safely switches to local storage

When reading an artifact, the tool first checks the local storage, and if absent, downloads the data from Redis and
saves it locally. When writing an artifact, it is saved simultaneously in both storages.

## Benefits

- Faster builds in CI/CD environments
- Reduced network and disk load
- Increased reliability through local backup
