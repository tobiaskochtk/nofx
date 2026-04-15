FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/selfhosted-ai500 ./cmd/selfhosted_ai500

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/selfhosted-ai500 /usr/local/bin/selfhosted-ai500
EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/selfhosted-ai500"]
