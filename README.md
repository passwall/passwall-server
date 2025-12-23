# PassWall Server

**PassWall Server** is the core backend for open source password manager PassWall platform. Using this server, you can safely store your passwords and access them from anywhere. 

[![License](https://img.shields.io/github/license/passwall/passwall-server)](https://github.com/passwall/passwall-server/blob/master/LICENSE)
[![GitHub issues](https://img.shields.io/github/issues/passwall/passwall-server)](https://github.com/passwall/passwall-server/issues)
[![Build Status](https://travis-ci.org/passwall/passwall-server.svg?branch=master)](https://travis-ci.org/passwall/passwall-server) 
[![Coverage Status](https://coveralls.io/repos/github/passwall/passwall-server/badge.svg?branch=master)](https://coveralls.io/github/passwall/passwall-server?branch=master)
[![Docker Pull Status](https://img.shields.io/docker/pulls/passwall/passwall-server)](https://hub.docker.com/u/passwall/)  
[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy)

## 📋 Table of Contents

- [Features](#-features)
- [Quick Start](#-quick-start)
- [Development](#-development)
- [Makefile Commands](#-makefile-commands)
- [Docker Deployment](#-docker-deployment)
- [Environment Variables](#-environment-variables)
- [API Documentation](#-api-documentation)
- [Security](#-security)
- [Support](#-support)

## ✨ Features

- 🔐 **Secure Password Storage** - AES-GCM encryption
- 🌐 **RESTful API** - Well-documented API endpoints
- 🐳 **Docker Support** - Easy deployment with Docker Compose
- 🔄 **Auto Migration** - Database schema management
- 📦 **Multiple Storage Types** - Passwords, credit cards, bank accounts, notes, emails
- 🛡️ **Security Middlewares** - XSS protection, SQL injection prevention, rate limiting
- 🎯 **JWT Authentication** - Secure token-based authentication

## 🚀 Quick Start

### Using Docker Compose (Recommended)

1. **Start the server:**
```bash
make up
```

2. **Create a new user:**
```bash
docker exec -it passwall-server /app/passwall-cli
```

3. **Access the server:**
```
Server URL: http://localhost:3625
```

### Using Docker Hub Image

```bash
# Create directory
mkdir $HOME/passwall-server
cd $HOME/passwall-server

# Download docker-compose.yml
wget https://raw.githubusercontent.com/passwall/passwall-server/main/build/docker/docker-compose.yml

# Start services
docker-compose up -d

# Create user
docker exec -it passwall-server /app/passwall-cli
```

## 💻 Development

### Prerequisites

- Go 1.24+ (or latest)
- PostgreSQL 13+
- Docker & Docker Compose (optional)
- Make

### Local Development Setup

1. **Clone the repository:**
```bash
git clone https://github.com/passwall/passwall-server.git
cd passwall-server
```

2. **Install dependencies:**
```bash
go mod download
```

3. **Install development tools:**
```bash
make install-tools
```

4. **Start PostgreSQL:**
```bash
make db-up
```

5. **Build and run:**
```bash
make run
```

### Development with Hot Reload

```bash
make dev
```

This will install and use [Air](https://github.com/air-verse/air) for automatic reloading on code changes.

## 📦 Makefile Commands

Run `make help` to see all available commands:

### General
```bash
make help              # Display help message
```

### Build
```bash
make build             # Build server and CLI binaries
make build-linux       # Build for Linux
make build-darwin      # Build for macOS
make build-all         # Build for all platforms
make clean             # Clean build artifacts
```

### Development
```bash
make generate          # Run go generate
make lint              # Run golangci-lint
make test              # Run tests
make test-coverage     # Run tests with coverage report
make install-tools     # Install development tools
```

### Local Development
```bash
make run               # Build and run server locally
make dev               # Run with hot reload (air)
make create-user       # Create a new user with CLI
```

### Docker
```bash
make image-build       # Build Docker image
make image-publish     # Build and publish to Docker Hub
```

### Docker Compose
```bash
make up                # Start all services (builds if needed)
make down              # Stop all services
make restart           # Restart all services
make logs              # Show logs
make ps                # Show running services
```

### Database
```bash
make db-up             # Start PostgreSQL only
make db-down           # Stop PostgreSQL
make db-logs           # Show PostgreSQL logs
```

### CI/CD
```bash
make ci                # Run full CI pipeline
make check             # Run lint and test
```

### Information
```bash
make version           # Show version information
make info              # Show build information
```

## 🐳 Docker Deployment

### Build Docker Image

```bash
make image-build
```

### Build and Publish to Docker Hub

```bash
# Login to Docker Hub first
docker login

# Build and publish
make image-publish
```

### Custom Docker Image Tag

```bash
DOCKER_TAG=v2.0.0 make image-build
DOCKER_TAG=v2.0.0 make image-publish
```

## 🔧 Environment Variables

### Server Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `3625` |
| `PW_SERVER_USERNAME` | Admin username | - |
| `PW_SERVER_PASSWORD` | Admin password | - |
| `PW_SERVER_PASSPHRASE` | Encryption passphrase | - |
| `PW_SERVER_SECRET` | JWT secret | - |
| `PW_SERVER_TIMEOUT` | Server timeout | `2` |
| `PW_SERVER_GENERATED_PASSWORD_LENGTH` | Generated password length | `16` |
| `PW_SERVER_ACCESS_TOKEN_EXPIRE_DURATION` | Access token expire duration | `30m` |
| `PW_SERVER_REFRESH_TOKEN_EXPIRE_DURATION` | Refresh token expire duration | `7d` |

### Database Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `PW_DB_NAME` | Database name | `passwall` |
| `PW_DB_USERNAME` | Database username | `postgres` |
| `PW_DB_PASSWORD` | Database password | `password` |
| `PW_DB_HOST` | Database host | `localhost` |
| `PW_DB_PORT` | Database port | `5432` |
| `PW_DB_LOG_MODE` | Enable DB logging | `false` |
| `PW_DB_SSL_MODE` | SSL mode | `disable` |

## 📚 API Documentation

API documentation is available at [Postman Public Directory](https://documenter.getpostman.com/view/3658426/SzYbyHXj)

## 🛡️ Security

1. **AES-GCM Encryption** - Passwords are encrypted with AES in Galois/Counter Mode. Passwords can only be decrypted with the passphrase defined in your configuration.

2. **Security Middlewares** - Endpoints are protected against XSS attacks and other common vulnerabilities.

3. **SQL Injection Prevention** - Using Gorm ORM which automatically sanitizes all queries.

4. **Rate Limiting** - Built-in rate limiter for signin attempts to prevent brute force attacks.

5. **JWT Authentication** - Secure token-based authentication with access and refresh tokens.

## 👥 Clients

**PassWall Server** can be used with:
- [**PassWall Desktop**](https://github.com/passwall/passwall-desktop)
- [**PassWall Extension**](https://github.com/passwall/passwall-extension)

<p align="center">
    <img src="https://www.yakuter.com/wp-content/yuklemeler/passwall-screenshot.png" alt="" width="600" height="425" />
</p>

## 💖 Support

I promise all the support will be spent on this project!

[![Become a Patron](https://www.yakuter.com/wp-content/yuklemeler/yakuter-patreon.png)](https://www.patreon.com/bePatron?u=33541638)

## 🤝 Contributing

### For Contributors

1. Don't send too many commits at once. It will be easier for us to do a code review.
2. Be sure to check out the `dev` branch. The latest development version is there.
3. First try to fix `// TODO:` items in the code.
4. Follow the milestones for feature development.
5. Don't modify the UI without design approval.

### Development Workflow

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run linter
make lint

# Run full CI pipeline
make ci
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🌟 Star History

If you like this project, please give it a ⭐ on GitHub!

---

Made with ❤️ by the PassWall Team
