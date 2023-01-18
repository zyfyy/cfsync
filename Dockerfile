FROM golang:alpine as builder
WORKDIR /app
COPY . .
RUN  go mod tidy
RUN go build .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/cfsync ./
CMD ./cfsync
