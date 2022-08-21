FROM registry.gitlab.com/etke.cc/base AS builder

WORKDIR /scheduler
COPY . .
RUN make build && \
    git clone https://gitlab.com/etke.cc/int/ansible-injector.git && \
    cd ansible-injector && \
    make build

FROM alpine:latest

ENV SCHEDULER_DB_DSN /data/scheduler.db

RUN apk --no-cache add ca-certificates tzdata olm ansible-core && update-ca-certificates && \
    adduser -D -g '' scheduler && \
    mkdir /data && chown -R scheduler /data

COPY --from=builder /scheduler/scheduler /bin/scheduler
COPY --from=builder /scheduler/ansible-injector/ansible-injector /bin/ansible-injector

USER scheduler

ENTRYPOINT ["/bin/scheduler"]

