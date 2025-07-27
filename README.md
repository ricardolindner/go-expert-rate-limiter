# go-expert-rate-limiter

---
## Project Structure
```text
rate-limiter-go/
|-- cmd/
|   |-- server/                 # Arquivo principal para iniciar o servidor HTTP
|       |-- main.go
|-- configs/                     # Leitura de variáveis de ambiente (.env)
|   |-- config.go
|-- internal/
|   |-- limiter/                # Lógica de rate limiting
|   |   |-- limiter.go
|   |   |-- strategy.go     # Interface para a estratégia de persistência (Redis, etc.)
|   |-- middleware/
|   |   |-- rate_limiter.go     # Middleware HTTP
|   |-- utils/
|       |-- key.go              # Funções para montar chaves IP/token
|-- pkg/
|   |-- server/                 # Setup do servidor HTTP
|       |-- router.go
|-- .env
|-- docker-compose.yml
|-- go.mod
|-- README.md
```