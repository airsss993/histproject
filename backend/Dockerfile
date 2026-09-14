FROM golang:1.26.0-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bootstrap-admin ./cmd/bootstrap-admin

FROM alpine:3.23.5

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S histproject \
    && adduser -S -G histproject histproject

ENV TZ=Asia/Yekaterinburg
WORKDIR /app

COPY --from=builder /out/ /app/

USER histproject
CMD ["/app/api"]
