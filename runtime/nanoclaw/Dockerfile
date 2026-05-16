FROM golang:1.25-bookworm AS runtime-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/shclop-runtime ./cmd/shclop-runtime

FROM node:24-bookworm-slim

RUN npm install -g nano-claw

COPY runtime/adapter/shclop-runtime-adapter.sh /usr/local/bin/shclop-runtime-adapter
COPY --from=runtime-build /out/shclop-runtime /usr/local/bin/shclop-runtime
RUN chmod +x /usr/local/bin/shclop-runtime-adapter \
    && addgroup --system --gid 10001 shclop \
    && adduser --system --uid 10001 --ingroup shclop shclop \
    && mkdir -p /workspace /memory \
    && chown -R 10001:10001 /workspace /memory

WORKDIR /workspace
ENV SHCLOP_AGENT_FLAVOR=nanoclaw
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/shclop-runtime-adapter"]
