FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

FROM alpine:3.19
# ffmpeg: video encoding (libx264, libfreetype, etc.)
# fontconfig + font-noto + ttf-freefont: required for drawtext filter
# msttcorefonts-installer → use noto as clean OSS alternative
RUN apk --no-cache add \
    ca-certificates \
    ffmpeg \
    fontconfig \
    font-noto \
    ttf-freefont && \
    fc-cache -f /usr/share/fonts
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
