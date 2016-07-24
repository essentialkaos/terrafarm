resource "digitalocean_droplet" "builder-x64" {
  image = "centos-7-0-x64"
  name = "terrafarm-c7-x64"
  region = "${var.region}"
  size = "${var.node_size}"
  ssh_keys = [
    "${var.fingerprint}"
  ]

  connection {
    user = "root"
    type = "ssh"
    key_file = "${var.key}"
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
      "yum -y -q install https://yum.kaos.io/7/release/x86_64/kaos-repo-7.0-0.el7.noarch.rpm",
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
    source = "conf/rpmmacros"
    destination = "/home/builder/.rpmmacros"
  }

  provisioner "file" {
    source = "conf/sudoers"
    destination = "/etc/sudoers"
  }
}
