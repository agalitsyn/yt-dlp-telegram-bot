# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based Telegram bot that downloads videos from various sources using yt-dlp and re-uploads them to Telegram using the MTProto API for larger file upload support (>50MB). The bot processes video/audio downloads, converts them if needed using FFmpeg, and uploads them to Telegram with a built-in video player compatible format.

## Architecture

The application is structured as a single Go module with the following key components:

- **main.go**: Entry point, Telegram client setup, message handling, and bot lifecycle management
- **queue.go**: Download queue management for processing requests sequentially
- **dl.go**: Core download logic using yt-dlp integration
- **convert.go**: Video/audio format conversion using FFmpeg
- **upload.go**: Telegram file upload handling with progress tracking
- **params.go**: Configuration parameter parsing from environment variables and command line
- **helper.go**: Utility functions for user/group resolution and messaging
- **vercheck.go**: yt-dlp version checking and auto-updates
- **cmd.go**: Command line argument parsing
- **rereader.go**: Custom reader for upload progress tracking

### Key Dependencies

- **github.com/gotd/td**: Telegram MTProto API client
- **github.com/wader/goutubedl**: yt-dlp Go wrapper
- **github.com/u2takey/ffmpeg-go**: FFmpeg Go bindings
- **github.com/flytam/filenamify**: Safe filename generation

## Development Commands

### Building
```bash
go build -v                    # Build the binary
go run *.go                    # Run from source
```

### Testing
```bash
go test ./...                  # Run all tests (if any exist)
go vet ./...                   # Static analysis
go fmt ./...                   # Format code
```

### Dependencies
```bash
go mod download                # Download dependencies
go mod tidy                    # Clean up dependencies
go mod verify                  # Verify dependencies
```

## Configuration

The bot requires configuration via environment variables or command line flags:

- **API_ID**: Telegram API ID
- **API_HASH**: Telegram API hash  
- **BOT_TOKEN**: Telegram bot token
- **ALLOWED_USERIDS**: Comma-separated allowed user IDs
- **ADMIN_USERIDS**: Comma-separated admin user IDs
- **ALLOWED_GROUPIDS**: Comma-separated allowed group IDs
- **MAX_SIZE**: Maximum file size limit (e.g., "512MB")
- **YTDLP_COOKIES**: Cookie file content for yt-dlp

Configuration is managed through `config.inc.sh` file and can be set via environment variables or command line arguments (see params.go:55-120).

## Deployment

### Local Development
```bash
cp config.inc.sh-example config.inc.sh
# Edit config.inc.sh with your values
bash run.sh
```

### Docker
```bash
docker compose build           # Build image
docker compose up             # Run container
```

The Docker setup uses a multi-stage build (golang:1.22 builder + python:alpine runtime) with FFmpeg included in the final image.

## Bot Commands

- `/dlp [mp3] <URL>`: Download video/audio from URL (mp3 prefix for audio-only)
- `/dlpcancel`: Cancel current download
- URLs sent in private chat are automatically processed as `/dlp` commands

## System Requirements

- yt-dlp (auto-downloaded if not found)
- FFmpeg and ffprobe (required for media conversion)
- Go 1.22+ for building from source