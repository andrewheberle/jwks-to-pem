FROM golang:1.24@sha256:87916acb3242b6259a26deaa7953bdc6a3a6762a28d340e4f1448e7b5c27c009 AS builder

COPY . /build

RUN cd /build && \
    go build ./cmd/jwks-to-pem

FROM gcr.io/distroless/base-debian12:nonroot@sha256:0a0dc2036b7c56d1a9b6b3eed67a974b6d5410187b88cbd6f1ef305697210ee2

COPY --from=builder /build/jwks-to-pem /app/jwks-to-pem

ENTRYPOINT [ "/app/jwks-to-pem" ]