FROM registry.gitlab.com/etke.cc/base AS builder

WORKDIR /postmoogle
COPY . .
RUN make build

FROM alpine:latest

ENV POSTMOOGLE_DB_DSN /data/postmoogle.db

RUN apk --no-cache add ca-certificates tzdata olm && \
    adduser -D -g '' postmoogle && \
    mkdir /data && chown -R postmoogle /data

COPY --from=builder /postmoogle/postmoogle /bin/postmoogle

USER postmoogle

ENTRYPOINT ["/bin/postmoogle"]

