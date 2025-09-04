FROM golang:1.22-alpine AS builder
WORKDIR /app/
COPY . .
RUN go build -v -mod=vendor


FROM python:3.13-alpine
RUN apk update && apk upgrade && apk add --no-cache ffmpeg
COPY --from=builder /app/yt-dlp-telegram-bot /app/yt-dlp-telegram-bot
RUN touch /app/yt-dlp-cookies.txt && chmod 644 /app/yt-dlp-cookies.txt
CMD ["/app/yt-dlp-telegram-bot"]
