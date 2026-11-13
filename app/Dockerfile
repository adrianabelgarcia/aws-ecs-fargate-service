# syntax=docker/dockerfile:1.7

# Both base images are pinned by tag AND digest (hadolint DL3006 requires an
# explicit tag). Dependabot (docker ecosystem) proposes digest bumps.

# ---- build ---------------------------------------------------------------
FROM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS build

WORKDIR /src

# Dependencies first: this layer is cached until go.mod or go.sum change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---- runtime -------------------------------------------------------------
# distroless/static: no shell, no package manager; CA certificates and tzdata only.
# The :nonroot variant runs as UID 65532. There is no HEALTHCHECK here: the ECS
# task definition runs `api healthcheck` instead, because this image has no shell.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS runtime

ARG SERVICE_VERSION=dev
ENV APP_ENV=prod \
    HTTP_PORT=8080 \
    METRICS_PORT=9090 \
    SERVICE_VERSION=${SERVICE_VERSION}

COPY --from=build /out/api /usr/local/bin/api

EXPOSE 8080 9090
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/api"]
