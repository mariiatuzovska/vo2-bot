FROM golang:1.25.2-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/bot

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/bot /usr/local/bin/bot

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/bot"]
