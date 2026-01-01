# Docker Files

This directory contains all Docker-related files for PassWall Server.

## Files

- **Dockerfile** - Multi-stage Docker build for PassWall Server
- **docker-compose.yml** - Full setup with PostgreSQL and PassWall Server
- **docker-compose-postgres.yml** - PostgreSQL only
- **docker-compose-passwall.yml** - PassWall Server only
- **docker-compose-nginx.yml** - Full setup with Nginx reverse proxy

## Usage

### Quick Start

From the project root:

```bash
# Start all services
make up

# Stop all services
make down

# View logs
make logs

# Start only database
make db-up
```

### Manual Docker Compose

From this directory:

```bash
# Start full stack
docker-compose up -d

# Start only PostgreSQL
docker-compose -f docker-compose-postgres.yml up -d

# Start with Nginx
docker-compose -f docker-compose-nginx.yml up -d
```

## Building Docker Image

From the project root:

```bash
# Build image
make image-build

# Build and publish to Docker Hub
make image-publish

# Custom tag
DOCKER_TAG=v2.0.0 make image-build
```

## Environment Variables

See main README.md for full list of environment variables.

### Common Variables

```yaml
environment:
  - PW_DB_NAME=passwall
  - PW_DB_USERNAME=postgres
  - PW_DB_PASSWORD=password
  - PW_DB_HOST=postgres
  - PW_DB_PORT=5432
  - PW_DB_LOG_MODE=false
  - PW_DB_SSL_MODE=disable
```

## Volumes

PassWall uses Docker named volumes for data persistence:

- **postgres_data** - PostgreSQL database files
- **passwall_data** - PassWall Server configuration

### Volume Management Commands

```bash
# List volumes
make volumes-list

# Inspect volumes
make volumes-inspect

# Backup volumes
make volumes-backup

# Clean all volumes (WARNING: deletes all data)
make volumes-clean
```

### Volume Location

Docker volumes are stored in Docker's data directory. To find them:

```bash
docker volume inspect docker_postgres_data
docker volume inspect docker_passwall_data
```

## Ports

- **3625** - PassWall Server API
- **5432** - PostgreSQL (if exposed)
- **80/443** - Nginx (if using nginx compose file)

## Notes

- The Dockerfile is optimized for production with multi-stage builds
- Uses Alpine Linux for minimal image size
- Binaries are built with CGO_ENABLED=0 for static linking
- Health checks are configured for PostgreSQL

