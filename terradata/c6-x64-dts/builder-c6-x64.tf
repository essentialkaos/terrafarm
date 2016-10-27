resource "digitalocean_droplet" "terrafarm-c6-x64" {
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
      "yum -y -q install https://yum.kaos.io/6/release/i386/kaos-repo-7.0-0.el6.noarch.rpm",
      "echo 'Installing EPEL repository package...'",
      "yum -y -q install epel-repo",
      "echo 'Updating packages...'",
      "yum -y -q update",
      "echo 'Installing DevToolSet repo...'",
      "rpm --import https://linux.web.cern.ch/linux/scientific6/docs/repository/cern/slc6X/i386/RPM-GPG-KEY-cern",
      "curl -o /etc/yum.repos.d/slc6-devtoolset.repo https://linux.web.cern.ch/linux/scientific6/docs/repository/cern/devtoolset/slc6-devtoolset.repo",
      "echo 'Installing RPMBuilder Node package...'",
      "yum -y -q install rpmbuilder-node",
      "echo 'Starting node configuration...'",
      "yum-config-manager --disable kaos-release &> /dev/null",
      "yum-config-manager --enable kaos-release-x64 &> /dev/null",
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
