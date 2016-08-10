FROM alpine:3.4
ADD consul-scheduler /usr/local/bin/consul-scheduler
ENTRYPOINT ["consul-scheduler", "combined"]
