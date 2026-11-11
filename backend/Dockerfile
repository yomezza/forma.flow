# syntax=docker/dockerfile:1
FROM golang:1.27.4-alpine AS build
# One build layer avoids copying the toolchain between intermediate snapshots.
RUN --mount=type=bind,source=.,target=/src,readonly \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=secret,id=proxy_ca,required=false \
    if [ -s /run/secrets/proxy_ca ]; then export SSL_CERT_FILE=/run/secrets/proxy_ca; fi; \
    cd /src && go mod download && \
    CGO_ENABLED=0 go build -mod=readonly -trimpath -o /out/server ./cmd/server && \
    CGO_ENABLED=0 go build -mod=readonly -trimpath -o /out/manage ./cmd/manage && \
    CGO_ENABLED=0 go build -mod=readonly -trimpath -o /out/bootstrap ./cmd/bootstrap && \
    cp /etc/ssl/certs/ca-certificates.crt /out/ca-certificates.crt && \
    cp -R migrations /out/migrations && \
    chmod -R a+rX /out
FROM scratch
WORKDIR /app
COPY --from=build /out/ /app/
ENV SSL_CERT_FILE=/app/ca-certificates.crt
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/server"]
