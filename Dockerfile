FROM alpine:3.4

ADD dist/sked_linux-arm64-latest /usr/local/bin/sked

ENTRYPOINT ["sked", "combined"]
