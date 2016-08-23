FROM alpine:3.4

ADD dist/linux /usr/local/bin/sked

ENTRYPOINT ["sked", "combined"]
