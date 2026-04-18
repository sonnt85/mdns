#!/usr/bin/env bash
# test_docker.sh — manual smoke test: relay between two docker networks.
# Not run in CI. Requires: docker, sudo, a built mdns-relay binary in PATH.
set -euo pipefail

RELAY_BIN="${RELAY_BIN:-mdns-relay}"
NET_A="mdns-relay-test-a"
NET_B="mdns-relay-test-b"

cleanup() {
    docker rm -f pub sub 2>/dev/null || true
    docker network rm "$NET_A" "$NET_B" 2>/dev/null || true
    [ -n "${RELAY_PID:-}" ] && kill "$RELAY_PID" 2>/dev/null || true
}
trap cleanup EXIT

echo "== creating docker networks =="
docker network create --driver bridge "$NET_A"
docker network create --driver bridge "$NET_B"

echo "== starting relay on host =="
sudo "$RELAY_BIN" -i "br-*" -w 2s -s 5s &
RELAY_PID=$!
sleep 3

echo "== publishing service in NET_A =="
docker run -d --rm --name pub --network "$NET_A" \
    -e AVAHI_SERVICE=_http._tcp \
    alpine:3.19 sh -c "apk add --no-cache avahi avahi-tools && \
        avahi-daemon --no-drop-root -D && \
        avahi-publish-service testsvc _http._tcp 80 && \
        sleep 600"

echo "== querying from NET_B =="
sleep 5
docker run --rm --name sub --network "$NET_B" \
    alpine:3.19 sh -c "apk add --no-cache avahi avahi-tools && \
        avahi-browse -rt _http._tcp" | tee /tmp/mdns-relay-test.out

if grep -q testsvc /tmp/mdns-relay-test.out; then
    echo "PASS: testsvc discovered across networks"
    exit 0
else
    echo "FAIL: testsvc not found in output above"
    exit 1
fi
