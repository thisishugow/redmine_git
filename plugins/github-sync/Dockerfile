# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# 安裝必要工具
RUN apk add --no-cache git ca-certificates tzdata

# 複製 go mod 檔案
COPY go.mod go.sum ./
RUN go mod download

# 複製程式碼
COPY . .

# 編譯
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o sync cmd/sync/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# 從 builder 複製執行檔
COPY --from=builder /build/sync .

# 建立 logs 目錄
RUN mkdir -p /app/logs

# 設定時區為台北
ENV TZ=Asia/Taipei

# 健康檢查（檢查 process 是否存活）
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD pgrep sync || exit 1

CMD ["./sync"]
