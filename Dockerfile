FROM golang:1.24.2-alpine3.20 as build-env
ARG BIN_NAME=site_audit
RUN apk --no-cache add build-base
WORKDIR /app
COPY go.mod go.sum ./

RUN go mod download && go mod verify
COPY . .

RUN make build
RUN chmod +x bin/$BIN_NAME

FROM alpine:3.20
ARG BIN_NAME=$BIN_NAME
RUN apk --no-cache add ca-certificates

COPY --from=build-env /app/bin/$BIN_NAME /$BIN_NAME
ENTRYPOINT ["/site_audit"]