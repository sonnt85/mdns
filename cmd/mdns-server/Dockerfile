FROM golang:1.25-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/mdns-server ./cmd/mdns-server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates iputils avahi-tools
COPY --from=build /out/mdns-server /usr/local/bin/mdns-server
EXPOSE 5353/udp
ENTRYPOINT ["/usr/local/bin/mdns-server"]
CMD ["-host", "abc", "-ip", "auto"]
