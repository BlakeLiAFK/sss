# SSS - Simple S3 Server

A lightweight, self-hosted S3-compatible object storage server with a built-in web management interface.

## Features

- **S3-Compatible API** - Works with AWS CLI, SDKs, and any S3-compatible client
- **AWS Signature V4** - Full support for standard AWS authentication
- **Presigned URLs** - Generate time-limited access URLs for objects
- **Multipart Upload** - Support for large file uploads (up to 5GB per object)
- **Web Management UI** - Modern Vue3-based admin interface
- **API Key Management** - Fine-grained bucket-level access control
- **Public/Private Buckets** - Configure bucket visibility
- **Single Binary** - Frontend embedded in Go binary, zero external dependencies
- **Cross-Platform** - Supports Linux, macOS, and Windows (amd64/arm64)
- **SQLite Storage** - Simple metadata storage, no external database required

## Quick Start

### Download

Download the latest release from [Releases](https://github.com/BlakeLiAFK/sss/releases) page.

#### Quick Install via wget/curl

**Linux (amd64)**

```bash
# 获取最新版本号
VERSION=$(curl -s https://api.github.com/repos/BlakeLiAFK/sss/releases/latest | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')

# 下载并解压
wget -qO- https://github.com/BlakeLiAFK/sss/releases/download/${VERSION}/sss-linux-amd64.tar.gz | tar xz

# 添加执行权限
chmod +x sss-linux-amd64
```

**Linux (arm64)**

```bash
VERSION=$(curl -s https://api.github.com/repos/BlakeLiAFK/sss/releases/latest | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
wget -qO- https://github.com/BlakeLiAFK/sss/releases/download/${VERSION}/sss-linux-arm64.tar.gz | tar xz
chmod +x sss-linux-arm64
```

**macOS (Apple Silicon)**

```bash
VERSION=$(curl -s https://api.github.com/repos/BlakeLiAFK/sss/releases/latest | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
curl -sL https://github.com/BlakeLiAFK/sss/releases/download/${VERSION}/sss-darwin-arm64.tar.gz | tar xz
chmod +x sss-darwin-arm64
```

**macOS (Intel)**

```bash
VERSION=$(curl -s https://api.github.com/repos/BlakeLiAFK/sss/releases/latest | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
curl -sL https://github.com/BlakeLiAFK/sss/releases/download/${VERSION}/sss-darwin-amd64.tar.gz | tar xz
chmod +x sss-darwin-amd64
```

**One-liner (Linux amd64)**

```bash
curl -sL $(curl -s https://api.github.com/repos/BlakeLiAFK/sss/releases/latest | grep 'browser_download_url.*linux-amd64.tar.gz' | cut -d'"' -f4) | tar xz && chmod +x sss-linux-amd64
```

### Run

```bash
# Linux/macOS
chmod +x sss-linux-amd64  # or sss-darwin-arm64
./sss-linux-amd64

# Windows
sss-windows-amd64.exe
```

The server starts at `http://localhost:8080`.

### First Run

On first access, you'll be guided through a setup wizard to configure:

- Admin username and password
- S3 region

## Configuration

### Command Line Arguments

All runtime configurations are set via command line arguments:

```bash
./sss [options]

Options:
  -host string    Listen address (default "0.0.0.0")
  -port int       Listen port (default 8080)
  -db string      Database path (default "./data/metadata.db")
  -data string    Data storage path (default "./data/buckets")
  -log string     Log level: debug/info/warn/error (default "info")
```

**Examples:**

```bash
# Custom port
./sss -port 9000

# Custom data directory
./sss -data /mnt/storage/buckets -db /mnt/storage/metadata.db

# Debug logging
./sss -log debug
```

### Web Settings (Runtime Configurable)

The following settings can be modified via Web UI → Settings:

| Setting         | Description                | Default            |
| --------------- | -------------------------- | ------------------ |
| S3 Region       | AWS region identifier      | us-east-1          |
| Max Object Size | Maximum single object size | 5 GB               |
| Max Upload Size | Presigned URL upload limit | 1 GB               |
| Admin Password  | Login password             | (set during setup) |

## S3 API Reference

### Supported Operations

| Category      | Operations                                                                                    |
| ------------- | --------------------------------------------------------------------------------------------- |
| **Bucket**    | ListBuckets, CreateBucket, DeleteBucket, HeadBucket                                           |
| **Object**    | GetObject, PutObject, DeleteObject, HeadObject, CopyObject                                    |
| **List**      | ListObjectsV1, ListObjectsV2                                                                  |
| **Multipart** | InitiateMultipartUpload, UploadPart, CompleteMultipartUpload, AbortMultipartUpload, ListParts |

### AWS CLI Configuration

```bash
# Configure AWS CLI
aws configure set aws_access_key_id YOUR_ACCESS_KEY
aws configure set aws_secret_access_key YOUR_SECRET_KEY
aws configure set region us-east-1

# Use with endpoint URL
aws --endpoint-url http://localhost:8080 s3 ls
aws --endpoint-url http://localhost:8080 s3 mb s3://my-bucket
aws --endpoint-url http://localhost:8080 s3 cp file.txt s3://my-bucket/
aws --endpoint-url http://localhost:8080 s3 ls s3://my-bucket
```

### Presigned URL Example

```bash
# Generate presigned URL (valid for 1 hour)
aws --endpoint-url http://localhost:8080 s3 presign s3://my-bucket/file.txt --expires-in 3600
```

## Web Management Interface

Access the web UI at `http://localhost:8080` after starting the server.

### Features

- **Dashboard** - Overview of storage usage and statistics
- **Bucket Management** - Create, delete, and configure buckets
- **Object Browser** - Upload, download, and manage objects
- **API Key Management** - Create and manage API keys with bucket-level permissions
  - Create API keys with descriptions
  - Set read/write permissions per bucket
  - Enable/disable API keys
  - Reset secret keys without recreating

### API Key Permissions

Each API key can have different permissions for different buckets:

| Permission     | Description                    |
| -------------- | ------------------------------ |
| Read           | List objects, download files   |
| Write          | Upload, delete, modify objects |
| `*` (Wildcard) | Access to all buckets          |

## Building from Source

### Prerequisites

- Go 1.21+
- Node.js 20+
- npm

### Build Commands

```bash
# Clone the repository
git clone https://github.com/BlakeLiAFK/sss.git
cd sss

# Build production binary (frontend embedded)
make build

# Build development version (frontend from filesystem)
make build-dev

# Run in development mode
make dev

# Clean build artifacts
make clean
```

## Project Structure

```
sss/
+-- cmd/server/          # Application entry point
+-- internal/
|   +-- api/             # HTTP handlers and routing
|   +-- auth/            # AWS Signature V4 implementation
|   +-- storage/         # File and metadata storage
|   +-- config/          # Configuration management
|   +-- utils/           # Utility functions
+-- web/                 # Vue3 frontend source
+-- data/                # Runtime data directory
|   +-- buckets/         # Object storage
|   +-- metadata.db      # SQLite database
+-- config.yaml          # Configuration file
```

## Security

### Authentication Flow

1. **S3 API** - AWS Signature V4 authentication
2. **Admin API** - Session-based authentication with JWT tokens
3. **Public Buckets** - Read-only access without authentication

### Best Practices

- Change default admin credentials before production use
- Use HTTPS in production (place behind a reverse proxy like Nginx or Caddy)
- Regularly rotate API keys
- Use least-privilege permissions for API keys

## API Endpoints

### Admin API

| Method | Endpoint                            | Description       |
| ------ | ----------------------------------- | ----------------- |
| POST   | /api/admin/login                    | Admin login       |
| POST   | /api/admin/logout                   | Admin logout      |
| GET    | /api/admin/apikeys                  | List API keys     |
| POST   | /api/admin/apikeys                  | Create API key    |
| DELETE | /api/admin/apikeys/:id              | Delete API key    |
| PUT    | /api/admin/apikeys/:id              | Update API key    |
| POST   | /api/admin/apikeys/:id/reset-secret | Reset secret key  |
| POST   | /api/admin/apikeys/:id/permissions  | Set permissions   |
| GET    | /api/admin/buckets                  | List buckets      |
| POST   | /api/admin/buckets                  | Create bucket     |
| DELETE | /api/admin/buckets/:name            | Delete bucket     |
| PUT    | /api/admin/buckets/:name/public     | Set public status |

### Custom S3 Extensions

| Method | Endpoint                 | Description            |
| ------ | ------------------------ | ---------------------- |
| POST   | /api/presign             | Generate presigned URL |
| GET    | /api/bucket/:name/search | Search objects         |

## Troubleshooting

### Common Issues

**Port already in use**

```bash
# Find process using port 8080
lsof -i :8080
# Kill the process
kill -9 <PID>
```

**Permission denied on data directory**

```bash
chmod -R 755 ./data
```

**AWS CLI signature errors**

- Verify access key and secret key
- Check system time synchronization
- Ensure endpoint URL is correct

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Changelog

### v1.0.1

- Added API Key reset secret functionality
- Fixed static file routing security vulnerability (SEC-001)

### v1.0.0

- Initial release
- Full S3-compatible API
- Web management interface
- API key permission system
- Cross-platform support
