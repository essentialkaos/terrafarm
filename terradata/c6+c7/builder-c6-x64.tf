resource "digitalocean_droplet" "builder-c6-x64" {
  image = "centos-6-5-x64"
  name = "terrafarm-c6-x64"
  region = "${var.region}"
  size = "${var.node_size}"
  ssh_keys = [
    "${var.fingerprint}"
  ]

  connection {
    user = "root"
    type = "ssh"
    private_key = "${file(var.key)}"
    timeout = "2m"
  }

  provisioner "remote-exec" {
    inline = [
      "export PATH=$PATH:/usr/bin",
      "echo 'Cleaning yum cache...'",
      "yum -y -q clean expire-cache",
      "echo 'Updating system packages...'",
      "yum -y -q update",
      "echo 'Installing KAOS repository package...'",
      "yum -y -q install https://yum.kaos.io/6/release/x86_64/kaos-repo-8.0-0.el6.noarch.rpm",
      "echo 'Updating packages...'",
      "yum -y -q update",
      "echo 'Installing RPMBuilder Node package...'",
      "yum -y -q install rpmbuilder-node",
      "echo 'Starting node configuration...'",
      "sed -i 's#builder:!!#builder:${var.auth}#' /etc/shadow",
      "echo 'Build node configuration complete'"
    ]
  }

  provisioner "file" {
    source = "conf/hosts.allow"
    destination = "/etc/hosts.allow"
  }

  provisioner "file" {
    source = "conf/c6-rpmmacros"
    destination = "/home/builder/.rpmmacros"
  }

  provisioner "file" {
    source = "conf/sudoers"
    destination = "/etc/sudoers"
  }
}
