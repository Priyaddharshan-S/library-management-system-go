# ---- Build stage ----
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy all source code first
COPY . .

# go.mod only declares the module name so far — go mod tidy scans the
# source files, figures out which packages are imported (gin, pgx,
# godotenv, etc.), and downloads + locks the right versions.
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/api

# ---- Run stage (small final image) ----
FROM alpine:3.20

WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8080
CMD ["./server"]
