FROM ubuntu:latest

ADD consul-scheduler /usr/local/bin/

CMD ["consul-scheduler", "combined"]
