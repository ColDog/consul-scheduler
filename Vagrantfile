# -*- mode: ruby -*-
# vi: set ft=ruby :

$install_consul = <<SCRIPT
  sudo apt-get install -y unzip curl
  sudo curl https://releases.hashicorp.com/consul/0.6.4/consul_0.6.4_linux_amd64.zip -o consul.zip
  unzip consul.zip && sudo chmod +x consul && sudo mv consul /usr/bin/consul
SCRIPT

Vagrant.configure(2) do |config|
  config.vm.box = "ubuntu/trusty64"

  config.vm.provision "docker"
  config.vm.synced_folder ".", "/vagrant", type: "nfs"

  config.vm.provider "virtualbox" do |v|
    v.customize [ "modifyvm", :id, "--cpus", "1" ]
    v.customize [ "modifyvm", :id, "--memory", "512" ]
  end

  config.vm.provision :shell, inline: 'sudo apt-get update'
  config.vm.provision :shell, inline: $install_consul
  config.vm.provision :shell, inline: 'docker pull coldog/sked'
  config.vm.provision :shell, inline: 'docker pull consul'
  config.vm.provision :shell, inline: 'docker rm -f consul || true'
  config.vm.provision :shell, inline: 'docker rm -f sked || true'

  3.times do |i|
    id = "n#{i + 1}"
    addr = "172.20.20.1#{i}"

    puts "id: #{id}, addr: #{addr}"

    config.vm.define id do |node|
      node.vm.hostname = id
      node.vm.network :private_network, ip: addr
      node.vm.provision :shell, inline: "docker run -d --net=host -e 'CONSUL_LOCAL_CONFIG={\"leave_on_terminate\": true}' --name=consul consul agent -node=#{id} -server -advertise=#{addr} -client=0.0.0.0"
      node.vm.provision :shell, inline: "docker run -d --net=host --name=sked coldog/sked"
      if id != "n1"
        node.vm.provision :shell, inline: "consul join 172.20.20.10"
      end
    end

  end

end
