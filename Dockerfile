# =============================================================================
# Stage 1: Tools — get a static busybox binary for container health checks
# =============================================================================
FROM alpine:3.19 AS tools
RUN apk add --no-cache busybox-static && \
    cp /bin/busybox.static /busybox.static

# =============================================================================
# Stage 2: Builder — compile the Go binary
# =============================================================================
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache ca-certificates
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /alvus .

# =============================================================================
# Stage 3: Runtime — minimal distroless image
# =============================================================================
FROM gcr.io/distroless/static:nonroot

# Copy CA certificates (needed for outbound HTTPS to upstream APIs)
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs/
# Copy the Go binary
COPY --from=builder /alvus /alvus
# Copy static busybox for HEALTHCHECK (wget -s spider mode)
COPY --from=tools /busybox.static /bin/busybox

EXPOSE 3000
USER nonroot:nonroot
ENTRYPOINT ["/alvus"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/bin/busybox", "wget", "-s", "http://localhost:3000/health"]