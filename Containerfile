FROM registry.access.redhat.com/hi/go:latest@sha256:666e77358fcda912f3251bc1651dc1908ea1d6e49c0eab43b0aacef272d187dc AS build
WORKDIR /src
COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    if [ -d "vendor" ]; then \
    echo "Using vendored dependencies" && \
    go mod verify; \
    else \
    echo "Downloading dependencies" && \
    go mod download && go mod verify; \
    fi

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    go build \
    -trimpath -buildvcs=false \
    -ldflags="-s -w -buildid= -extldflags='-static'" \
    -o /out/sigcull ./cmd/sigcull

FROM registry.access.redhat.com/hi/static:latest@sha256:20f419d12511f96524d9b9bb092ef5066d6bacee7ed45d7c528bef62f6d48f74 AS final
COPY --from=build /out/sigcull /usr/local/bin/sigcull

# config.yaml and the secret env vars are provided at run time.
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/sigcull", "-config", "/etc/sigcull/config.yaml"]
