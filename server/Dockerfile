# syntax=docker/dockerfile:1.7

FROM golang:1.23-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod ./
# go.sum может отсутствовать на раннем этапе; если есть — будет скопирован отдельно
COPY go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/server ./cmd/app

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/server /app/server

EXPOSE 8080
ENTRYPOINT ["/app/server"]
