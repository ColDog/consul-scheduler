FROM alpine:3.4

ADD dist/sked_linux-amd64-latest /usr/local/bin/sked

CMD ["sked", "combined"]
