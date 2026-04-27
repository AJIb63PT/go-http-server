# go-http-server

URL shortener service written in Go with SQLite storage and in-memory caching.

![hippo](https://github.com/AJIb63PT/go-http-server/blob/main/main.go-http-go-server-Visual-Studio-Code-2026-04-27-09-54-43.gif)


## Setup

### 1. Create `.env` file

Create a `.env` file in the root directory with the following content:

**Linux/macOS:**
```bash
echo "CONFIG_PATH=config/local.yaml" > .env
```

**Windows (PowerShell):**
```powershell
"CONFIG_PATH=config/local.yaml" | Out-File -Encoding UTF8 .env
```

**Windows (Command Prompt):**
```cmd
echo CONFIG_PATH=config/local.yaml > .env
```

### 2. Install dependencies

```bash
go mod download
```

### 3. Run the server

```bash
go run ./cmd/url-shortener
```

The server will start on `http://localhost:8082` (or the address specified in `config/local.yaml`).

## API Endpoints

### Create short link
```bash
curl -X POST http://localhost:8082/links \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com","shortCode":"mylink"}'
```

### Get link
```bash
curl -X GET http://localhost:8082/links/mylink
```

### Delete link
```bash
curl -X DELETE http://localhost:8082/links/mylink
```

### List links (with pagination)
```bash
curl -X GET "http://localhost:8082/links?limit=10&offset=0"
```

### Get link stats
```bash
curl -X GET http://localhost:8082/links/mylink/stats
```

## Features

- **In-memory caching**: Frequently accessed links are cached with 1-minute TTL
- **SQLite storage**: Persistent data storage
- **Pagination support**: List links with limit/offset parameters
- **Link statistics**: Track visits and creation timestamp
- **Cache introspection**: API responses show cache source and TTL information
