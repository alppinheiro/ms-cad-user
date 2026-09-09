# syntax=docker/dockerfile:1
# =============================================================================
#  Build multi-target: escolhe o binário pelo ARG TARGET (padrão do monorepo)
#    docker build --build-arg TARGET=cmd/worker -t ms-cad-user-worker .
#    docker build --build-arg TARGET=cmd/generator -t ms-cad-user-generator .
# =============================================================================
FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGET=cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./${TARGET}

FROM alpine:3.20
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
