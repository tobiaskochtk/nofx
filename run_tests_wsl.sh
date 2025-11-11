#!/bin/bash
# Script to run Go tests in WSL with proper environment setup

export GOROOT=/home/bot/.local/go
export PATH=$GOROOT/bin:/home/bot/.local/bin:$PATH
export GOTOOLCHAIN=local
export CGO_ENABLED=1

cd /mnt/c/Code/nofx || exit 1

echo "Go version:"
go version

echo ""
echo "Running tests with proper permissions..."
echo ""

# Fix permissions if needed
echo 'bot' | sudo -S chmod -R 755 decision_logs/ 2>/dev/null

# Run tests
go test ./... -v
