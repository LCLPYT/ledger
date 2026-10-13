FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS build
ARG TARGETOS TARGETARCH

WORKDIR /src

ARG GRPC_HEALTH_PROBE_VERSION=v0.4.59
ARG GRPC_HEALTH_PROBE_SHA256_AMD64=b74ea9e3fe0e4fb19008a77422b6475e29141b30833a6f82c18941bf6eb6a136
ARG GRPC_HEALTH_PROBE_SHA256_ARM64=f6d3e6a62734a66bab0584bd008897c2b7183a6cf0a5003082820ae2970f24d7

RUN case "${TARGETARCH}" in \
        amd64) sum="${GRPC_HEALTH_PROBE_SHA256_AMD64}" ;; \
        arm64) sum="${GRPC_HEALTH_PROBE_SHA256_ARM64}" ;; \
        *) echo "unsupported architecture ${TARGETARCH}" >&2 && exit 1 ;; \
    esac \
    && mkdir -p /out && wget -qO /out/grpc_health_probe \
        "https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-${TARGETARCH}" \
    && echo "${sum}  /out/grpc_health_probe" | sha256sum -c - \
    && chmod +x /out/grpc_health_probe

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/ledger .

# Run
FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3

ARG VERSION=dev
ARG REVISION=unknown

LABEL org.opencontainers.image.title="ledger" \
      org.opencontainers.image.source="https://github.com/lclpyt/ledger" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.licenses="AGPL-3.0-only"

COPY --from=build /out/ledger /usr/local/bin/ledger
COPY --from=build /out/grpc_health_probe /bin/grpc_health_probe

# default ports
EXPOSE 50051 8080

# default healthcheck, assuming default ports
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/bin/grpc_health_probe", "-addr=127.0.0.1:50051"]

ENTRYPOINT ["/usr/local/bin/ledger"]
