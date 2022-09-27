#!/usr/bin/env bash

env GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o foolish-mysql cmd/foolish-mysql/main.go
