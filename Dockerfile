FROM golang:1.27@sha256:820153096fda4de6ac1b7194d12e416b7dd8f4e9a33d17e0345b111b945162d8 AS builder

COPY . /build

RUN cd /build && \
    go build ./cmd/jwks-to-pem

FROM gcr.io/distroless/base-debian12:nonroot@sha256:0a0dc2036b7c56d1a9b6b3eed67a974b6d5410187b88cbd6f1ef305697210ee2

COPY --from=builder /build/jwks-to-pem /app/jwks-to-pem

ENTRYPOINT [ "/app/jwks-to-pem" ]