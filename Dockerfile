FROM registry.gitlab.com/etke.cc/base/build AS builder

WORKDIR /postmoogle
COPY . .
RUN just build

FROM registry.gitlab.com/etke.cc/base/app

ENV POSTMOOGLE_DB_DSN /data/postmoogle.db

COPY --from=builder /postmoogle/postmoogle /bin/postmoogle

USER app

ENTRYPOINT ["/bin/postmoogle"]

