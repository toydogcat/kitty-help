# Build stage: get official binary
FROM cloudflare/cloudflared:latest as builder

# Final stage: Alpine-based to support sh, grep, etc.
FROM alpine:3.23

COPY --from=builder /usr/local/bin/cloudflared /usr/local/bin/cloudflared

RUN apk add --no-cache \
    bash \
    grep \
    ca-certificates \
    tzdata \
    curl

RUN chmod +x /usr/local/bin/cloudflared

WORKDIR /root
ENTRYPOINT ["/usr/local/bin/cloudflared"]
