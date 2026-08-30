FROM registry.access.redhat.com/hi/go:latest@sha256:a605c12fbeffffe1ebed9a58ca77c6d575e303be9726bbd689a45d212f0465bc AS build
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

FROM registry.access.redhat.com/hi/static:latest@sha256:f4d5109b57cf7eab0a7adc566f2d78f80fa0c5ec9ccab698c9fb8eb448db6071 AS final
COPY --from=build /out/sigcull /usr/local/bin/sigcull

# config.yaml and the secret env vars are provided at run time.
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/sigcull", "-config", "/etc/sigcull/config.yaml"]
