# ==========================================
# Builder Stage (Go + Rust + Node)
# ==========================================
FROM debian:bookworm-slim AS builder

WORKDIR /app

# 1. Install Toolchain
RUN apt-get update && apt-get install -y \
    curl git make build-essential \
    protobuf-compiler libprotobuf-dev \
    binaryen brotli \
    pkg-config libssl-dev \
    && rm -rf /var/lib/apt/lists/*

# Install Go 1.21+
COPY --from=golang:1.24-bookworm /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

# Install Rust (via rustup)
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"
RUN rustup target add wasm32-unknown-unknown

# Install Node.js 20+
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs

# Install Cap'n Proto 1.0+ (Source or apt if new enough, bookworm has 0.9+, might need source for 1.0 features if used)
# For now relying on apt's capnproto if available or just the go plugins
RUN apt-get update && apt-get install -y capnproto

# 2. Copy Source
COPY . .

# 3. Setup & Dependencies
# Install Go/Capnp tools
RUN make install-tools

# 4. Build All (Kernel -> Modules -> Frontend)
# This runs the standard Makefile flow which puts everything in 'frontend/dist'
RUN make all

# ==========================================
# Runtime Stage (Nginx)
# ==========================================
FROM fholzer/nginx-brotli:v1.24.0

# Copy Static Assets
COPY --from=builder /app/frontend/dist /usr/share/nginx/html

# Copy Config
COPY frontend/nginx/nginx.conf /etc/nginx/nginx.conf

# Expose HTTP
EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
