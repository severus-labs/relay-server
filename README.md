# Relay Server

A minimal Go server for ephemeral encrypted data sharing. Designed as the backend component for [Severus](https://github.com/severus-labs/severus) secure sharing functionality.

## Features

- **Ephemeral storage** - Data expires automatically after 10 minutes
- **Rate limiting** - 10 requests per minute per IP address
- **SQLite backend** - Local database with automatic cleanup
- **Zero-knowledge design** - Server only stores encrypted blobs
- **Single binary deployment** - No external dependencies

## Quick Start

```bash
# Clone and build
git clone https://github.com/severus-labs/relay-server.git
cd relay-server
go mod tidy
go build -o relay-server

# Run server
./relay-server
```

Server starts on port 8080 by default. Set `PORT` environment variable to change.

## API Endpoints

### Health Check

```bash
GET /health
```

Returns server status.

### Check Code Availability

```bash
GET /check/{code}
```

- **404** - Code available
- **409** - Code already in use

### Share Data

```bash
POST /share
Content-Type: application/json

{
    "code": "ABC1-DEF2-GHI3",
    "data": "base64-encoded-encrypted-data",
    "expires_minutes": 10
}
```

### Receive Data

```bash
GET /receive/{code}
```

Returns encrypted data if code exists and hasn't expired.

## Configuration

- **Port**: Set `PORT` environment variable (default: 8080)
- **Database**: SQLite database created as `./relay.db`
- **Rate Limiting**: 10 requests/minute per IP (hardcoded)
- **Expiration**: 10 minutes maximum (configurable per request, max 60 minutes)

## Security Model

- Server never sees plaintext data - only encrypted blobs
- Automatic cleanup removes expired shares every 30 seconds
- Rate limiting prevents abuse
- No persistent user accounts or authentication required
- Shares are accessible to anyone with the code during validity period

## Deployment

<!-- ### Docker (coming soon)

```bash
docker run -p 8080:8080 severuslabs/relay-server
``` -->

### Binary Deployment

1. Build: `go build -o relay-server`
2. Copy binary to server
3. Run with process manager (systemd, supervisor, etc.)

### Environment Variables

- `PORT` - Server port (default: 8080)

## Development

```bash
# Install dependencies
go mod tidy

# Run in development
go run main.go

# Build
go build -o relay-server

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o relay-server-linux
```

## License

MIT License - see [LICENSE](LICENSE) file.

## Related Projects

- [Severus](https://github.com/severus-labs/severus) - Local-first encrypted vault CLI tool
