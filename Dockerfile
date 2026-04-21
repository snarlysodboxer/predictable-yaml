FROM golang:1.24-alpine AS builder
RUN apk add --no-cache bash curl
WORKDIR /go/src/github.com/snarlysodboxer/predictable-yaml
COPY go.* ./
RUN go mod download
COPY . .
RUN bash hack/fetch-default-configs.sh
ARG VERSION=dev
RUN GOOS=linux CGO_ENABLED=0 go build -ldflags "-X github.com/snarlysodboxer/predictable-yaml/cmd.Version=${VERSION}" -o /predictable-yaml

FROM alpine:3.21 AS app
COPY --from=builder /predictable-yaml /predictable-yaml
ENTRYPOINT ["/predictable-yaml"]
