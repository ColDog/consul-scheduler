# -*- mode: ruby -*-
# vi: set ft=ruby :

# Vagrantfile API/syntax version. Don't touch unless you know what you're doing!
VAGRANTFILE_API_VERSION = "2"

Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|
  config.vm.box = "ubuntu/trusty64"

  config.vm.provision "docker"
  config.vm.synced_folder ".", "/vagrant", type: "nfs"

  config.vm.provision :shell, inline: 'docker rm -f consul || true'
  config.vm.provision :shell, inline: 'docker rm -f consul-scheduler || true'
  config.vm.provision :shell, inline: 'docker pull coldog/consul-scheduler'

  config.vm.define "n1" do |n1|
    n1.vm.hostname = "n1"
    n1.vm.network "private_network", ip: "172.20.20.10"
    n1.vm.provision :shell, inline: 'docker run -d --net=host -e \'CONSUL_LOCAL_CONFIG={"leave_on_terminate": true}\' --name=consul consul agent -node=n1 -server -bind=172.20.20.10'
    n1.vm.provision :shell, inline: 'docker run -d --net=host --name=consul-scheduler coldog/consul-scheduler'
  end

  config.vm.define "n2" do |n2|
    n2.vm.hostname = "n2"
    n2.vm.network "private_network", ip: "172.20.20.11"
    n2.vm.provision :shell, inline: 'docker run -d --net=host -e \'CONSUL_LOCAL_CONFIG={"leave_on_terminate": true}\' --name=consul consul agent -node=n2 -server -bind=172.20.20.11 -bootstrap-expect=1'
    n2.vm.provision :shell, inline: 'docker run -d --net=host --name=consul-scheduler coldog/consul-scheduler'
    n2.vm.provision :shell, inline: 'consul join 172.20.20.10'
  end
end
