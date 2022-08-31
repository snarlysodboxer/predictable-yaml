FROM golang:1.19.0-alpine AS builder
WORKDIR /go/src/github.com/snarlysodboxer/predictable-yaml
COPY go.* ./
RUN go mod download
COPY . .
RUN GOOS=linux CGO_ENABLED=0 go build -o /predictable-yaml

FROM scratch AS app
COPY --from=builder /predictable-yaml /predictable-yaml
ENTRYPOINT ["/predictable-yaml"]
