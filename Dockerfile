FROM golang:1.24@sha256:10c131810f80a4802c49cab0961bbe18a16f4bb2fb99ef16deaa23e4246fc817 AS builder

COPY . /build

RUN cd /build && \
    go build ./cmd/jwks-to-pem

FROM gcr.io/distroless/base-debian12:nonroot@sha256:06c713c675e983c5aea030592b1d635954218d29c4db2f8ec66912da1b87e228

COPY --from=builder /build/jwks-to-pem /app/jwks-to-pem

ENTRYPOINT [ "/app/jwks-to-pem" ]