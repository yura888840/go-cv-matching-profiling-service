# Go CV Matching Profiling Service

A Go/Gin HTTP service for ranking CVs against a vacancy stored in PostgreSQL, with a deterministic scoring engine, configurable worker count, IP/Origin filtering, API-key authentication, and reproducible CPU/RSS benchmarks. The project is a **prototype**, not a production-ready matching system.

## Components

- `cmd/api`: Gin HTTP server (POST matching, health and readiness endpoints).
- `internal/service`: `MatchingService` coordinating PostgreSQL reads and matching.
- `internal/storage/postgres`: pgxpool-based repository.
- `internal/matching`: deterministic integer scoring, sequential and worker-pool execution, Top K ranking.
- `cmd/matcher`: simple synthetic CLI comparison.
- `cmd/profilematch`: Go heap/CPU pprof data generator (JSON metrics).
- `cmd/genapikey`: securely generates a 256-bit API key using `crypto/rand`.
- `scripts/compare.py`: repeatable Linux process-level CPU/RSS/throughput comparison, standalone binary (no PostgreSQL).
- `scripts/render_charts.py`: rebuilds the committed SVG benchmark charts from `results.json`.

## Requirements

- Go **1.23+** and Python **3.9+** for the comparison/chart scripts (standard library only).
- Docker Compose for local PostgreSQL 17, or your own PostgreSQL instance.
- For charts of process CPU/RSS: **Linux** (`os.wait4`/`ru_maxrss`). The Go profiler itself runs on macOS/Windows too.
- Free network access on first `go mod tidy` / `go mod download` to fetch Gin, pgx, and godotenv.

## Quick start (local development)

```bash
cp .env.example .env
# Generate a secret and put its output into API_KEY in .env:
go run ./cmd/genapikey
# Start PostgreSQL and run the bundled SQL migration on first startup:
docker compose up -d
# Resolve module dependencies:
go mod tidy
# Run HTTP API:
go run ./cmd/api
```

The initial migration is in `migrations/001_init.sql`. The Compose Postgres container executes it **only during initial database creation**. If you already have a populated volume, apply the migration manually.

### `.env` configuration

```dotenv
DATABASE_URL=postgres://matcher:matcher@localhost:5432/matcher?sslmode=disable
PORT=8080
MAX_CVS=500000
MATCHING_WORKERS=4
ALLOWED_IPS=127.0.0.1,::1
ALLOWED_DOMAINS=app.example.com
API_KEY=<your-generated-secret>
```

- `MATCHING_WORKERS`: default `4`, valid `1..64` (per request, not global concurrency cap). JSON request `workers` overrides this setting; `mode: sequential` runs on one goroutine.
- `MAX_CVS`: maximum number of active CVs read from DB per request, default `500000`. This is a potentially expensive full-table read: apply SQL prefiltering before production.
- `ALLOWED_IPS`: allowed socket peer IPv4/IPv6 addresses/CIDRs; `ALLOWED_DOMAINS`: exact browser `Origin` hosts. Both empty denies all traffic (including health checks).
- **Security limitation:** `Origin` can be spoofed by non-browser clients and is **not** authentication. Behind Nginx/Load Balancer, Go sees the proxy's IP; apply client-IP allowlisting at the trusted proxy instead. Use HTTPS in production. Never commit `.env` or make API keys accessible to browsers.

## Seed PostgreSQL

```bash
docker compose exec -T postgres psql -U matcher -d matcher <<'SQL'
INSERT INTO vacancies(required_skills,min_years,region,max_salary)
VALUES (7,2,1,100000);
INSERT INTO cvs(skills,years,region,salary)
VALUES (7,5,1,80000),(3,4,1,70000),(1,1,1,50000);
SQL
```

## API usage

Set `API_KEY` in the shell to the **same key** in `.env`, then request matching:

```bash
curl -sS -X POST http://localhost:8080/api/v1/matches \
  -H 'Content-Type: application/json' \
  -H "X-API-Key: $API_KEY" \
  -d '{"vacancy_id":1,"top":100,"mode":"worker_pool","cv_limit":100000}'
```

Example response (illustrative):

```json
{
  "data": {
    "vacancy_id": 1,
    "evaluated": 3,
    "matches": [{"CVID": 1, "Score": 100}]
  },
  "duration_ms": 2.4
}
```

`CVID` and `Score` currently use Go's exported field names (no JSON tags). Scores range from 0 to 100; they are a heuristic, not probabilities. `matches` is Top N sorted by descending score, then ascending CV ID. `evaluated` counts CVs read into memory. `duration_ms` measures the handler's matching operation including DB reads. The API returns IDs and scores, not candidate personal details.

- `GET /health` → `200 {"status":"ok"}` (liveness; allowlist applies, API key not required).
- `GET /ready` → `200 {"status":"ready","database":"ok"}` or `503 {"status":"not_ready","database":"unavailable"}` (database ping; allowlist applies).
- `POST /api/v1/matches` → `200` matching result; `400` invalid request, `401` invalid API key, `403` disallowed client, `404` missing vacancy, `500` server/database error.

## Unit tests and Go microbenchmarks

```bash
go test ./...
go test -race ./internal/matching ./internal/service
go test ./internal/matching -run '^$' \
  -bench 'Benchmark(Sequential|WorkerPool)$' -benchmem -count=3 -benchtime=2s
```

`ns/op`, `B/op`, and `allocs/op` are Go benchmark metrics; **`B/op` is allocated bytes per operation, not peak RSS.** Avoid comparing microbenchmark metrics to full HTTP throughput.

## CPU and heap profiling (pprof)

```bash
mkdir -p profiles
go build -o profiles/profilematch ./cmd/profilematch
GOMAXPROCS=4 ./profiles/profilematch \
  -mode sequential -cvs 500000 -iterations 200 \
  -cpuprofile profiles/sequential.cpu -memprofile profiles/sequential.heap
GOMAXPROCS=4 ./profiles/profilematch \
  -mode worker_pool -workers 4 -cvs 500000 -iterations 200 \
  -cpuprofile profiles/workers4.cpu -memprofile profiles/workers4.heap

go tool pprof -http=:8081 profiles/sequential.cpu
go tool pprof -http=:8082 profiles/workers4.cpu
go tool pprof -http=:8083 profiles/workers4.heap
```

The profiler CLI prints JSON: wall time, CV/s, allocation count/bytes, before/after Go heap, and profile paths. **`peak_heap_bytes` is 0 because instantaneous Go peak heap is not measured**. Heap profiles are snapshots, not peak RSS. It excludes synthetic data generation from timed matching, but synthetic data remain resident. `profiles/` is gitignored.

## Reproducible Linux CPU and RAM comparison

```bash
GOMAXPROCS=4 python3 scripts/compare.py \
  --cvs 500000 --iterations 200 --samples 3 \
  --out docs/benchmarks/results.json
python3 scripts/render_charts.py
```

The comparison compiles `cmd/profilematch` once, warms up, alternates modes, and records each process's elapsed wall time, user+system CPU time, average CPU utilization relative to **one core** (100% = one fully utilized logical CPU), and **peak resident set size (RSS)**. It uses Linux `os.wait4` CPU/RSS counters and doesn't require extra Python packages. These are **process peaks/averages**, not Go heap peak or instantaneous CPU peak. Compilation, PostgreSQL, HTTP, networking and application concurrency are excluded. RAM includes generated synthetic CV data. `GOMAXPROCS=4` controls available Go execution parallelism; server CPU quotas may still constrain it.

### Visual results — measured in this environment

Measurement: Linux amd64, Go 1.23.2, 500,000 synthetic CVs × 200 iterations per process, Top 100, four logical Go CPUs, 3 samples per mode, results below show medians. Benchmark output and system identity are checked into [`docs/benchmarks/results.json`](docs/benchmarks/results.json). **Run the commands on your own machine to compare your hardware.**

![Matching duration: sequential vs 4 workers](docs/benchmarks/wall-time.svg)

![Average CPU utilisation: sequential vs 4 workers](docs/benchmarks/cpu.svg)

![Peak RSS: sequential vs 4 workers](docs/benchmarks/memory.svg)

`CPU %` in the chart is **average usage** across the process lifetime, not instantaneous maximum; `Peak RSS` is an OS-reported peak. More workers can lower elapsed time while increasing aggregate CPU consumption and memory allocation. Performance is hardware-, scheduler-, and dataset-dependent. No SQL/HTTP work is measured.

## Production limitations

This is a synthetic matching benchmark and simple read-only API. It currently reads up to `MAX_CVS` records per request, uses 64-bit skill masks rather than normalized real-world skills, and the per-request worker count does not cap total HTTP concurrency. For production, implement DB prefilters, request rate and concurrency limits, configurable weighted scoring, observability, migrations, and authorization that checks whether a caller may access the vacancy/CVs.
