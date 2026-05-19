FROM node:22-alpine AS web-build
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/shclop ./cmd/shclop

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=go-build /out/shclop /usr/local/bin/shclop
WORKDIR /app/config
COPY config/identity.mock.yaml ./
WORKDIR /app
COPY --from=web-build /src/web/dist /app/web/dist
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/shclop"]
