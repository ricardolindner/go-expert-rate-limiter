# go-expert-rate-limiter

A simple and extensible HTTP rate limiter written in Go, using Redis as the backend for request counting and blocking. This project allows you to limit the number of requests per IP or per API token, with customizable limits and block times.

---

## Table of Contents
- [Project Structure](#project-structure)
- [How It Works](#how-it-works)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
  - [Environment Variables (.env)](#environment-variables-env)
  - [Example .env File](#example-env-file)
- [Running the Project](#running-the-project)
- [Testing the Rate Limiter](#testing-the-rate-limiter)
- [Unit and Integration tests](#unit-and-integration-tests)
- [Extending](#extending)

---

## Project Structure

```text
go-expert-rate-limiter/
|-- cmd/
|   |-- server/                 # Main entrypoint for the HTTP server
|       |-- [main.go]
|-- configs/                    # Loads environment variables (.env)
|   |-- [config.go]
|-- internal/
|   |-- limiter/                # Rate limiting logic
|   |   |-- [limiter.go]
|   |   |-- [strategy.go]       # Storage strategy interface (e.g., Redis)
|   |-- middleware/
|   |   |-- [rate_limiter.go]   # HTTP middleware for rate limiting
|   |-- repository/
|   |   |-- [redis.go]          # Redis implementation for storage
|-- .env                        # Environment variables (not committed)
|-- [docker-compose.yml]        # Docker Compose for Redis
|-- [go.mod]
|-- [README.md]
```

## How It Works
* The server applies rate limiting to all incoming HTTP requests;
* Limits can be set globally (per IP) or per API token;
* When the limit is exceeded, the client receives a 429 Too Many Requests response with a Retry-After header;
* Limits and block times are configurable via environment variables.

## Getting Started
Prerequisites
* Go 1.18+
* Docker
* Docker Compose

Clone the repository
```bash
git clone https://github.com/ricardolindner/go-expert-rate-limiter.git
cd go-expert-rate-limiter
```

Download the dempendencies:
```bash
go mod tidy
```

## Configuration
All configuration is done via environment variables.
You must create a .env file in the project root.

### Environment Variables (.env)

**Main variables**
* `REDIS_ADDR`: Redis server address (default: localhost:6379)
* `REDIS_PASSWORD`: Redis password (default: empty)
* `REDIS_DB`: Redis database number (default: 0)
* `DEFAULT_MAX_REQUESTS`: Default max requests per IP/token (default: 10)
* `DEFAULT_BLOCK_TIME_MINUTES`: Default block time in minutes (default: 1)

**Per-token limits**
To set custom limits for API tokens, use:
* `TOKEN_<TOKEN>_MAX`: Max requests for token `<TOKEN>`
* `TOKEN_<TOKEN>_BLOCK_MINUTES`: Block time in minutes for token `<TOKEN>`

Example: For token `MYTOKEN`, set `TOKEN_MYTOKEN_MAX` and `TOKEN_MYTOKEN_BLOCK_MINUTES`.

### Example .env File
```.env
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

DEFAULT_MAX_REQUESTS=5
DEFAULT_BLOCK_TIME_MINUTES=1

TOKEN_MY_SECRET_TOKEN_MAX=100
TOKEN_MY_SECRET_TOKEN_BLOCK_MINUTES=5

TOKEN_ANOTHER_TOKEN_MAX=20
TOKEN_ANOTHER_TOKEN_BLOCK_MINUTES=1
```

## Running the Project
### 1. Start Redis
In the project root path start the Redis:
```bash
docker-compose up -d
```

### 2. Build and Run the Server
In the project root path start the server:
```bash
go run ./cmd/server/main.go
```
The server will start on port 8080.

## Testing the Rate Limiter

### 1. Testing by IP
Make requests for the server URL `http://localhost:8080/`.
It's possible to run via `CURL` or using Postman or Insomnia:
```bash
for i in $(seq 1 30); do curl -v http://localhost:8080/; done
```

### 1. Testing by TOKEN
To test the token limit you must add the `API_KEY` header:
```bash
for i in $(seq 1 30); do curl -v -H "API_KEY: ANOTHER_TOKEN" http://localhost:8080/; done
```

## Unit and Integration tests
The integration tests use `testcontainers`, so the docker running is a must have.
In the project root run the tests:
```bash
go test ./...
```
Expected output:
```bash
?       github.com/ricardolindner/go-expert-rate-limiter/cmd/server
?       github.com/ricardolindner/go-expert-rate-limiter/configs
ok      github.com/ricardolindner/go-expert-rate-limiter/internal/limiter
?       github.com/ricardolindner/go-expert-rate-limiter/internal/repository
ok      github.com/ricardolindner/go-expert-rate-limiter/internal/middleware
```

## Extending
* To add new storage backends, implement the `LimiterStorage` interface in `strategy.go`.