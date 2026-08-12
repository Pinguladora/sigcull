# Build stage.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Fully static: verification and cloning run in-process (no git or gitsign
# binaries at runtime), so the runtime image can be distroless/static.
RUN CGO_ENABLED=0 go build -trimpath -o /out/sigcull ./cmd/sigcull

# Runtime stage. No git, no gitsign: everything runs in-process.
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/sigcull /usr/local/bin/sigcull

# config.yaml and the secret env vars are provided at run time.
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/sigcull", "-config", "/etc/sigcull/config.yaml"]
