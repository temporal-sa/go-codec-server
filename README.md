# Go Codec Server

A Temporal payload codec server implementation in Go that provides data compression and JWT-based authentication for Temporal Cloud workflows.

## Overview

This project consists of three main components:
- **Codec Server**: HTTP server that handles payload encoding/decoding with JWT authentication
- **Worker**: Temporal worker that processes workflows with compressed payloads
- **Starter**: Client application that starts workflow executions

Note: The codec server uses the public temporal cloud JWKS endpoint `https://login.tmprl.cloud/.well-known/jwks.json` to validate user tokens forwarded from temporal cloud

## Prerequisites

Before getting started, ensure you have the following tools installed:

- [Go](https://golang.org/doc/install) (1.23.0 or later)
- [tcld](https://docs.temporal.io/cloud/tcld) (Temporal Cloud CLI)
- [mkcert](https://github.com/FiloSottile/mkcert) (for local SSL certificates)

## Quick Start

### 1. Generate Certificates

Use the provided script to generate all required certificates:

```bash
./generate-certs
```

This script will:
- Create a `codec-server/certs/` directory
- Generate CA certificate and key using `tcld`
- Generate client certificate using `tcld`
- Generate server certificate using `mkcert`

### 2. Configure Temporal Cloud Namespace

The CA certificate must be added to your Temporal Cloud namespace for authentication to work.

**Option A: Using Cloud UI**
1. Navigate to the [Temporal Cloud UI](https://docs.temporal.io/cloud/certificates#update-certificates-using-temporal-cloud-ui)
2. Go to your namespace settings
3. Upload the CA certificate: `codec-server/certs/ca.pem`

**Option B: Using tcld CLI**
```bash
tcld namespace accepted-client-ca set --ca-certificate-file codec-server/certs/ca.pem
```

### 3. Set Environment Variables

Copy the environment template and configure your settings:

```bash
cp .env.example .env
source .env
```

### 4. Run the Components

Start each component in separate terminal windows:

**Terminal 1 - Codec Server:**
```bash
go run codec-server/main.go -cert codec-server/cets/localhost+1.pem -key codec-server/certs/localhost+1-key.pem -web "*"
```

**Terminal 2 - Worker:**
```bash
go run worker/main.go
```

**Terminal 3 - Starter:**
```bash
go run starter/main.go
```

## Configuration

### Environment Variables

The worker accepts the following environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `TLS_CERT_PATH` | Path to client certificate | `""` |
| `TLS_KEY_PATH` | Path to client key | `""` |
| `TEMPORAL_HOST_PORT` | Temporal server endpoint | `localhost:7233` |
| `TEMPORAL_NAMESPACE` | Temporal namespace | `default` |

### Codec Server Options

The codec server supports these command-line flags:

- `-port`: Server port (default: 8081)
- `-web`: Temporal Web URL for CORS support
- `-cert`: Path to TLS certificate for HTTPS
- `-key`: Path to TLS key for HTTPS

## Viewing Workflow Results

### Temporal Cloud UI

Navigate to your namespace in the Temporal Cloud UI to view workflow executions with decoded payloads.

Note: Use the codec-server section in the UI to set your codec-server's URL and ensure that the "Pass user access token" option is selected. Requests to the codec server without a valid JWT token are rejected.

## Features

- **Payload Compression**: Uses Snappy compression algorithm to reduce payload size
- **JWT Authentication**: Validates JWT tokens using JWKS from Temporal Cloud
- **Namespace Support**: Supports multiple namespaces with different codec configurations
- **CORS Support**: Optional CORS support for Temporal Web integration
- **TLS Support**: Optional HTTPS support with custom certificates

## Architecture

```
┌─────────────┐    ┌──────────────┐    ┌─────────────────┐
│   Starter   │───▶│    Worker    │───▶│ Temporal Cloud  │
└─────────────┘    └──────────────┘    └─────────────────┘
                           │                      │
                           ▼                      ▼
                   ┌──────────────┐    ┌─────────────────┐
                   │ Codec Server │◀───│ Temporal Web UI │
                   └──────────────┘    └─────────────────┘
```

## Troubleshooting

### Common Issues

1. **Certificate errors**: Ensure CA certificate is properly uploaded to your namespace
2. **Connection issues**: Verify your Temporal Cloud endpoint and credentials
3. **JWT validation errors**: Check that your namespace is properly configured for JWT authentication

### Logs

The codec server logs all requests and JWT validation details. Check the server output for:
- JWT token validation status
- Subject, Issuer, and Audience information
- Payload processing results
