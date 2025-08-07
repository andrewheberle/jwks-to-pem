FROM golang:1.24@sha256:10c131810f80a4802c49cab0961bbe18a16f4bb2fb99ef16deaa23e4246fc817 AS builder

COPY . /build

RUN cd /build && \
    go build ./cmd/jwks-to-pem

FROM gcr.io/distroless/base-debian12:nonroot@sha256:dca858878977f01b80685240ca2cd9c1bb38958e2ab6f4ed5698c0ea23307143

COPY --from=builder /build/jwks-to-pem /app/jwks-to-pem

ENTRYPOINT [ "/app/jwks-to-pem" ]