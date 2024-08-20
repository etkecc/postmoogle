FROM ghcr.io/etkecc/base/build AS builder

WORKDIR /app
COPY . .
RUN just build

FROM scratch

ENV POSTMOOGLE_DB_DSN /data/postmoogle.db

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/postmoogle /bin/postmoogle

USER app

ENTRYPOINT ["/bin/postmoogle"]

