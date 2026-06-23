
FROM registry.suse.com/bci/golang:1.26 AS builder

ARG MK_HOST_ARCH
ENV ARCH=$MK_HOST_ARCH
ENV GOTOOLCHAIN=auto

RUN zypper -n rm container-suseconnect 2>/dev/null || true && \
    zypper -n install git curl gzip tar wget awk

# Copy golangci-lint binary from a multi-arch digest, zero-trust
COPY --from=golangci/golangci-lint:v2.12.2-alpine@sha256:91b27804074a0bacea298707f016911e60cf0cdbc6c7bf5ccacb5f0606d18d60 /usr/bin/golangci-lint /usr/local/bin/golangci-lint

## install controller-gen
RUN GO111MODULE=on go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.1

ENV HOME=/go/src/github.com/harvester/harvester-load-balancer


# ---- base ----
FROM builder AS base
WORKDIR /go/src/github.com/harvester/harvester-load-balancer

# to exclude some files, add them in .dockerignore
COPY . .


# ---- build ----
FROM base AS build
ARG MK_REPO_ID

RUN --mount=type=cache,target=/go/pkg/mod,id=harvester-go-mod-${MK_REPO_ID} \
    --mount=type=cache,target=/go/src/github.com/harvester/harvester-load-balancer/.cache/go-build,id=harvester-go-build-${MK_REPO_ID} \
    ./scripts/build

FROM scratch AS build-output
COPY --from=build /go/src/github.com/harvester/harvester-load-balancer/bin/ /bin/


# ---- validate ----
FROM base AS validate
ARG MK_REPO_ID

RUN --mount=type=cache,target=/go/pkg/mod,id=harvester-go-mod-${MK_REPO_ID} \
    --mount=type=cache,target=/go/src/github.com/harvester/harvester-load-balancer/.cache/go-build,id=harvester-go-build-${MK_REPO_ID} \
    ./scripts/validate


# ---- test ----
FROM base AS test
ARG MK_REPO_ID

RUN --mount=type=cache,target=/go/pkg/mod,id=harvester-go-mod-${MK_REPO_ID} \
    --mount=type=cache,target=/go/src/github.com/harvester/harvester-load-balancer/.cache/go-build,id=harvester-go-build-${MK_REPO_ID} \
    ./scripts/test


# ---- test-integration ----
FROM base AS test-integration


# ---- generate-manifest ----
FROM base AS generate-manifest
RUN ./scripts/generate-manifest

FROM scratch AS generate-manifest-output
COPY --from=generate-manifest /go/src/github.com/harvester/harvester-load-balancer/crds/ /
