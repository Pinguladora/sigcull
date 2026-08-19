FROM reg.mini.dev/go:1.26@sha256:34f341e7bcc8181eeebcedbe4e2a0585e68aa89a01e70bd79e6d8642a8e03b6e AS build
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

FROM gcr.io/distroless/static-debian13:nonroot@sha256:f7f8f729987ad0fdf6b05eeeae94b26e6a0f613bdf46feea7fc40f7bd72953e6 AS final
COPY --from=build /out/sigcull /usr/local/bin/sigcull

# config.yaml and the secret env vars are provided at run time.
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/sigcull", "-config", "/etc/sigcull/config.yaml"]
