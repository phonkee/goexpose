#!/usr/bin/env bash

version=$(grep -F "VERSION = " settings.go | cut -d\" -f2)

echo "Cross compiling goexpose version: $version"

echo "Compiling for linux-amd64..."
env GOOS=linux GOARCH=amd64 go build -ldflags "-s" -o build/goexpose-linux-amd64-$version ./cmd/goexpose
echo "Compiling for linux-arm64..."
env GOOS=linux GOARCH=arm64 go build -ldflags "-s" -o build/goexpose-linux-arm64-$version ./cmd/goexpose
echo "Compiling for darwin-amd64..."
env GOOS=darwin GOARCH=amd64 go build -ldflags "-s" -o build/goexpose-darwin-amd64-$version ./cmd/goexpose
echo "Compiling for freebsd-amd64..."
env GOOS=freebsd GOARCH=amd64 go build -ldflags "-s" -o build/goexpose-freebsd-amd64-$version ./cmd/goexpose
echo "Compiling for windows-amd64..."
env GOOS=windows GOARCH=amd64 go build -ldflags "-s" -o build/goexpose-windows-amd64-$version.exe ./cmd/goexpose
