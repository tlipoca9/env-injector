FROM golang:1.23-alpine3.20 as builder
RUN apk add make

WORKDIR /build

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN make build

FROM ghcr.io/flant/shell-operator:v1.4.11
COPY --from=builder /build/bin/ /hooks/