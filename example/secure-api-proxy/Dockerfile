# Build Stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy the entire workspace to build the module and example
COPY . .

# Build the proxy binary statically
RUN go build -o /sf-proxy ./example/secure-api-proxy

# Final Stage
FROM alpine:latest

# Security: Run as non-privileged user
RUN addgroup -S sfproxy && adduser -S sfproxy -G sfproxy
USER sfproxy

COPY --from=builder /sf-proxy /usr/local/bin/sf-proxy

# Configuration
ENV PROXY_CONFIG=/etc/sf-proxy/config.yaml

# Volume for certificates and config
VOLUME ["/etc/sf-proxy"]

# Proxy typically listens on 8443 (HTTPS)
EXPOSE 8443

ENTRYPOINT ["/usr/local/bin/sf-proxy"]
