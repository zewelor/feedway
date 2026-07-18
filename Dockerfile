FROM --platform=$BUILDPLATFORM golang:1.26.5-trixie@sha256:117e07f49461abb984fc8aef661432461ff43d06faa22c3b73af6a49ce325cb9 AS dependencies

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

FROM gcr.io/distroless/static-debian13:nonroot@sha256:f7f8f729987ad0fdf6b05eeeae94b26e6a0f613bdf46feea7fc40f7bd72953e6 AS runtime

COPY --from=build --chown=nonroot:nonroot /feedway /feedway
COPY --chown=nonroot:nonroot LICENSE /LICENSE

EXPOSE 80

USER nonroot:nonroot

ENTRYPOINT ["/feedway"]
