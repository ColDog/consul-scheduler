FROM ubuntu


RUN apt-get update
RUN apt-get install -y unzip curl

RUN curl https://releases.hashicorp.com/consul/0.6.4/consul_0.6.4_linux_amd64.zip -o consul.zip
RUN unzip consul.zip && chmod +x consul && mv consul /usr/bin/consul

RUN mkdir -p /var/lib/consul

RUN apt-get install -y nginx curl

ADD sked_linux-amd64-latest /usr/local/bin/sked

CMD ["sked", "combined"]
