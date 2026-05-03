# ── Build Stage ───────────────────────────────────────────────────────────────
FROM golang:1.26.2-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o favofeeder ./cmd/favofeeder

# ── Runtime Stage ─────────────────────────────────────────────────────────────
# Node.js is required to run the OpenAI Codex CLI (@openai/codex).
FROM node:22-bookworm-slim

WORKDIR /app

# Install OpenAI Codex CLI globally
RUN npm install -g @openai/codex

# Copy compiled Go binary
COPY --from=builder /app/favofeeder /usr/local/bin/favofeeder

# Mount point for targets.yaml (provided via volume at runtime)
VOLUME ["/app/config"]

ENTRYPOINT ["favofeeder"]
CMD ["-config", "/app/config/targets.yaml"]
