FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder

ARG TARGETOS TARGETARCH
ENV CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH

ARG VERSION=dev

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY internal ./internal
COPY *.go ./

RUN go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o pushport .

FROM alpine:3.22

RUN apk add --no-cache tini ca-certificates

WORKDIR /app

COPY --from=builder /workspace/pushport /usr/local/bin/pushport

VOLUME ["/app/data"]

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q --spider http://127.0.0.1:8080/healthz

ENTRYPOINT ["/sbin/tini", "--", "pushport"]

CMD ["serve"]
