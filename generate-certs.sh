#!/bin/bash

# Script to generate certificates for the codec server
# This script generates:
# 1. CA certificate and key using tcld
# 2. Client/leaf certificate using tcld
# 3. Server certificate using mkcert

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if required tools are installed
check_tools() {
    local missing_tools=()

    if ! command -v tcld &> /dev/null; then
        missing_tools+=("tcld")
    fi

    if ! command -v mkcert &> /dev/null; then
        missing_tools+=("mkcert")
    fi

    if [ ${#missing_tools[@]} -ne 0 ]; then
        print_error "The following required tools are not installed:"
        for tool in "${missing_tools[@]}"; do
            echo "  - $tool"
        done
        echo ""
        print_info "Please install the missing tools and try again."
        print_info "For tcld: Visit https://docs.temporal.io/cloud/tcld"
        print_info "For mkcert: Visit https://github.com/FiloSottile/mkcert"
        exit 1
    fi
}

print_info "Checking for required tools..."
check_tools

# Define the certs directory
CERTS_DIR="codec-server/certs"

# Create the certs directory if it doesn't exist
if [ ! -d "$CERTS_DIR" ]; then
    print_info "Creating certs directory: $CERTS_DIR"
    mkdir -p "$CERTS_DIR"
else
    print_warn "Certs directory already exists. Existing certificates may be overwritten."
fi

# Change to the certs directory
cd "$CERTS_DIR"

print_info "Generating certificates in $(pwd)..."

# Generate CA certificate and key
print_info "Generating CA certificate and key..."
if tcld gen ca --org temporal -d 1y --ca-cert ca.pem --ca-key ca.key; then
    print_info "CA certificate generated successfully"
else
    print_error "Failed to generate CA certificate"
    exit 1
fi

# Generate leaf/client certificate
print_info "Generating leaf/client certificate..."
if tcld gen leaf --org temporal -d 364d --ca-cert ca.pem --ca-key ca.key --cert client.pem --key client.key; then
    print_info "Client certificate generated successfully"
else
    print_error "Failed to generate client certificate"
    exit 1
fi

# Generate server certificate using mkcert
print_info "Generating server certificate with mkcert..."
if mkcert localhost 127.0.0.1; then
    print_info "Server certificate generated successfully"
else
    print_error "Failed to generate server certificate"
    exit 1
fi

print_info "Certificate generation complete!"
print_info "Generated certificates:"
ls -la *.pem *.key 2>/dev/null || print_warn "No certificate files found"

echo ""
print_info "Certificates are located in: $(pwd)"
