FROM golang:1.22 AS build-image

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY /cmd/*.go ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -ldflags="-s -w" main.go

FROM alpine:latest

WORKDIR /app

COPY --from=build-image /app/main ./

ENTRYPOINT ["/app/main"]
