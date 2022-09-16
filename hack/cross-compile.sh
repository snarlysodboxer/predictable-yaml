#!/bin/bash

GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -o predictable-yaml-linux-amd64
GOARCH=amd64 GOOS=darwin CGO_ENABLED=0 go build -o predictable-yaml-darwin-amd64
GOARCH=arm64 GOOS=darwin CGO_ENABLED=0 go build -o predictable-yaml-darwin-arm64
