FROM golang:1.23 AS build-image

# Install certificates to fix error: "tls: failed to verify certificate: x509: certificate signed by unknown authority"
RUN apt-get update && apt-get install -y ca-certificates openssl
ARG cert_location=/usr/local/share/ca-certificates
# Get certificate from "github.com"
RUN openssl s_client -showcerts -connect github.com:443 </dev/null 2>/dev/null|openssl x509 -outform PEM > ${cert_location}/github.crt
# Get certificate from "proxy.golang.org"
RUN openssl s_client -showcerts -connect proxy.golang.org:443 </dev/null 2>/dev/null|openssl x509 -outform PEM >  ${cert_location}/proxy.golang.crt
# Get certificate from "storage.googleapis.com"
RUN openssl s_client -showcerts -connect storage.googleapis.com:443 </dev/null 2>/dev/null|openssl x509 -outform PEM >  ${cert_location}/storage.googleapis.crt

# Update certificates
RUN update-ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY /cmd/*.go ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -ldflags="-s -w" -o main *.go

FROM alpine:latest

WORKDIR /app

COPY --from=build-image /app/main ./

ENTRYPOINT ["/app/main"]
