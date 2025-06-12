FROM mirror.gcr.io/golang:1.24 as builder
WORKDIR /tmp
COPY . .
RUN echo foo > /tmp/bar

FROM mirror.gcr.io/busybox:latest AS modifier
WORKDIR /tmp
COPY --from=builder /tmp/bar /tmp/bar
RUN echo foo2 >> /tmp/bar

FROM mirror.gcr.io/busybox:latest
WORKDIR /
COPY --from=modifier /tmp/bar /bin/baz
COPY dir /var/dir

RUN echo /bin/baz
