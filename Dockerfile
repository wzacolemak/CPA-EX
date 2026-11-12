ARG RUNTIME_IMAGE=debian:bookworm

FROM golang:1.26-bookworm AS builder

# Keep the official proxy as the default while allowing restricted networks to
# select a reachable mirror with --build-arg GOPROXY=... .
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
ENV GOTOOLCHAIN=auto

WORKDIR /app


COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=1 GOOS=linux go build -buildvcs=false -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" -o ./CLIProxyAPI ./cmd/server/

FROM ${RUNTIME_IMAGE}


RUN mkdir -p /CLIProxyAPI

# Runtime bases such as debian:bookworm ship without ca-certificates, and Go's
# TLS then rejects every public HTTPS upstream with "certificate signed by
# unknown authority" - all configured providers fail at once, which reads like
# a network outage. Copy the Mozilla bundle from the builder stage instead of
# installing it, so the image stays offline-buildable and small.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder ./app/CLIProxyAPI /CLIProxyAPI/CLIProxyAPI

COPY config.example.yaml /CLIProxyAPI/config.example.yaml
# Pinned management asset with this fork's API-key quota extension.
COPY static/management.html /CLIProxyAPI/static/management.html

WORKDIR /CLIProxyAPI

EXPOSE 8317

ENV TZ=Asia/Shanghai

RUN cp /usr/share/zoneinfo/${TZ} /etc/localtime && echo "${TZ}" > /etc/timezone

# Fail the build, not production, when the trust store is missing or unusable:
# an image without a CA bundle only breaks at the first upstream request, long
# after the deploy looked successful. The size/content check runs only when grep
# is present, so a minimal runtime base without it cannot false-positive; the
# non-empty test above is the check that actually catches this bug.
RUN test -s /etc/ssl/certs/ca-certificates.crt || { echo "ERROR: runtime image has no CA bundle at /etc/ssl/certs/ca-certificates.crt; Go TLS will reject every public HTTPS upstream" >&2; exit 1; }
RUN if command -v grep >/dev/null 2>&1; then grep -q "BEGIN CERTIFICATE" /etc/ssl/certs/ca-certificates.crt || { echo "ERROR: CA bundle at /etc/ssl/certs/ca-certificates.crt has no certificate; Go TLS will reject every public HTTPS upstream" >&2; exit 1; }; fi

CMD ["./CLIProxyAPI"]
