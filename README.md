# gocacheprog

A high-performance build cache helper for Go that shares compiler artifacts across builds and CI/CD runners using **Redis** or **Valkey**. Implements the official [GOCACHEPROG protocol](https://pkg.go.dev/cmd/go/internal/cacheprog) introduced in Go 1.24.

Used in production, accelerating large project builds in GitLab CI/CD **by 2–4×**.

---

## Features

- **Blazing Fast**: Up to **1.6+ GB/s** cache throughput with concurrent pipelining.
- **Two-Tier Storage**: Local disk cache for zero-latency hits + remote Redis/Valkey for cross-build cache sharing.
- **Zstandard Compression (`-compress`)**: Transparent on-the-fly compression powered by `klauspost/compress/zstd` with zero-allocation pooling.
- **Zero-Downtime Resilience**: Seamless fallback to local disk if Redis/Valkey is unreachable — builds never fail due to network or cache outages.
- **Universal Driver**: Works out-of-the-box with **Standalone Redis/Valkey**, **Redis Cluster**, and **Redis Sentinel**.

---

## Performance & Benchmarks

Simulated incremental build benchmark (200 packages, 70% cache hit rate, Apple M4 Pro):

| Metric | Local Disk Cache | Disk + Valkey + Zstandard |
| :--- | :---: | :---: |
| **Throughput** | **1,595 MB/s** | **1,631 MB/s** |
| **Pipeline Latency** | **27.6 ms** | **26.9 ms** |
| **Memory Allocated** | ~18 MB | ~69 MB |

* Built-in Zstandard pooling (`sync.Pool`) reduces memory allocations by **89%** during compression, preventing GC pressure and OOMs on large builds.
* Direct string reading avoids memory duplication for high-frequency cache downloads.

---

## Requirements

* **Go 1.24+** (recommended, native `GOCACHEPROG` support)
* **Go 1.23** (requires `GOEXPERIMENT=gocacheprog`)
* Redis 6+ or Valkey 7+/8+/9+ (Standalone, Cluster, or Sentinel)

---

## Installation

```bash
 go install github.com/yonesko/gocacheprog@latest
```

---

## Usage

Set the `GOCACHEPROG` environment variable when invoking `go build`, `go test`, or `go install`:

```bash
GOCACHEPROG="gocacheprog -r-urls localhost:6379 -dir /tmp/gocache -compress" go build ./...
```

### Command Line Options

| Flag | Default | Description |
| :--- | :---: | :--- |
| `-dir` | *(required)* | Local directory for primary disk cache |
| `-r-urls` | `""` | Comma-separated list of Redis/Valkey addresses (e.g. `redis1:6379,redis2:6379`) |
| `-compress` | `false` | Enable Zstandard compression for remote cache entries |
| `-r-prefix` | `""` | Key prefix for Redis/Valkey cache entries |
| `-r-usr` | `""` | Redis/Valkey username (optional) |
| `-r-pwd` | `""` | Redis/Valkey password (optional) |
| `-log-metrics` | `0` | Metrics logging level: `0` (disabled), `1` (timing), `2` (detailed stats) |
| `-log-req` | `false` | Log incoming protocol requests to `stderr` |
| `-log-resp` | `false` | Log outgoing protocol responses to `stderr` |
| `-cpuprofile` | `""` | Path to write CPU profile |
| `-traceprofile`| `""` | Path to write runtime execution trace |

---

## Architecture

```
                  ┌──────────────────┐
                  │     go build     │
                  └────────┬─────────┘
                           │ stdin / stdout (JSON protocol)
                           ▼
                  ┌──────────────────┐
                  │   gocacheprog    │
                  └────┬────────┬────┘
                       │        │
          (sync hit)   ▼        ▼  (async background upload)
        ┌──────────────────┐  ┌──────────────────┐
        │  Local Disk Cache│  │  Redis / Valkey  │
        │      (-dir)      │  │  (-r-urls, zstd) │
        └──────────────────┘  └──────────────────┘
```

1. **GET Request**: Checks the local disk first. On cache miss, fetches from Redis/Valkey, decompresses, and copies to local disk.
2. **PUT Request**: Writes to local disk immediately and schedules an asynchronous upload to Redis/Valkey in the background.
3. **Fault Tolerance**: If Redis/Valkey fails or times out during startup handshake, `gocacheprog` automatically falls back to local disk storage with a warning.

---

## GitLab CI / CD Example

```yaml
build:
  stage: build
  variables:
    GOCACHE_DIR: "/tmp/gocache"
    REDIS_ADDR: "valkey.internal:6379"
  before_script:
    - mkdir -p "$GOCACHE_DIR"
  script:
    - GOCACHEPROG="gocacheprog -dir $GOCACHE_DIR -r-urls $REDIS_ADDR -compress" go build ./...
```
