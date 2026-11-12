FROM --platform=$BUILDPLATFORM golang:1.27.4-trixie@sha256:b54b2639de8c1bfe538ec5ffc45f29b1932235ac5f6d63c059853646677112d9 AS dependencies

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

FROM dependencies AS tools

WORKDIR /tools

COPY tools/go.mod tools/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod verify && \
    go mod tidy -diff && \
    go build -o /usr/local/bin/golangci-lint \
        github.com/golangci/golangci-lint/v2/cmd/golangci-lint && \
    go build -o /usr/local/bin/govulncheck \
        golang.org/x/vuln/cmd/govulncheck

FROM dependencies AS source

COPY cmd ./cmd
COPY internal ./internal

FROM source AS ci

RUN go mod verify && go mod tidy -diff

COPY --from=tools /usr/local/bin/golangci-lint /usr/local/bin/golangci-lint
COPY --from=tools /usr/local/bin/govulncheck /usr/local/bin/govulncheck

CMD ["go", "test", "-race", "-tags=integration", "./..."]

FROM source AS build

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /feedway ./cmd/feedway

FROM scratch AS runtime

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /feedway /feedway
COPY LICENSE /LICENSE

EXPOSE 80

USER 65532:65532

ENTRYPOINT ["/feedway"]
