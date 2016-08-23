# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure(2) do |config|
  config.vm.box = "ubuntu/trusty64"

  config.vm.provision "docker"
  config.vm.synced_folder ".", "/vagrant", type: "nfs"

  config.vm.provision :shell, inline: 'docker pull coldog/sked'
  config.vm.provision :shell, inline: 'docker pull consul'
  config.vm.provision :shell, inline: 'docker rm -f consul || true'
  config.vm.provision :shell, inline: 'docker rm -f consul-scheduler || true'

  3.times do |i|
    id = i + 1

    config.vm.define "n#{id}" do |node|
      node.vm.hostname = "n#{id}"
      node.vm.network "private_network", ip: "172.20.20.1#{id - 1}"
      node.vm.provision :shell, inline: "docker run -d --net=host -e 'CONSUL_LOCAL_CONFIG={\"leave_on_terminate\": true}' --name=consul consul agent -node=n#{id} -server -bind=172.20.20.1#{id}"
      node.vm.provision :shell, inline: "docker run -d --net=host --name=sked coldog/sked"
      if id != 1
        node.vm.provision :shell, inline: "consul join 172.20.20.10"
      end
    end

  end

end
