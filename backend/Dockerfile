FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/suidu-api ./cmd/server

FROM alpine:3.22
RUN adduser -D -H -u 10001 suidu
USER suidu
COPY --from=build /out/suidu-api /usr/local/bin/suidu-api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/suidu-api"]

