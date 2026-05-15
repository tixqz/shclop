FROM node:24-bookworm-slim

RUN npm install -g nano-claw

COPY runtime/adapter/shclop-runtime-adapter.sh /usr/local/bin/shclop-runtime-adapter
RUN chmod +x /usr/local/bin/shclop-runtime-adapter \
    && addgroup --system --gid 10001 shclop \
    && adduser --system --uid 10001 --ingroup shclop shclop \
    && mkdir -p /workspace /memory \
    && chown -R 10001:10001 /workspace /memory

WORKDIR /workspace
ENV SHCLOP_AGENT_FLAVOR=nanoclaw
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/shclop-runtime-adapter"]
