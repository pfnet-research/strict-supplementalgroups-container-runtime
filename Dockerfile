FROM golang:1.17 as builder
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/
COPY Makefile Makefile
RUN make build-only

FROM ubuntu:22.04
RUN mkdir -p /opt/strict-supplementalgroups-container-runtime/bin
COPY --from=builder /app/dist /opt/strict-supplementalgroups-container-runtime/bin
ENTRYPOINT [ "/opt/strict-supplementalgroups-container-runtime/bin/strict-supplementalgroups-install" ]
