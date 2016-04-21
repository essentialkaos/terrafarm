## TerraFarm

`terrafarm` is utility for working with terraform based rpmbuilder farm on [DigitalOcean](https://www.digitalocean.com).

#### Installation

To build the terrafarm from scratch, make sure you have a working Go 1.5+ workspace ([instructions](https://golang.org/doc/install)), then:

```
go get github.com/essentialkaos/terrafarm
```

If you want update terrafarm to latest stable release, do:

```
go get -u github.com/essentialkaos/terrafarm
```

#### Configuration

`terrafarm` use two ways for farm preconfiguration — preferences file and command-line arguments. Preferences file use next format:

```yaml
prop-name: prop-value
```

Example:

```yaml
user: builder
token: abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234
key: /home/user/.ssh/terra-farm
output: /home/user/terrafarm-nodes.list
region: ams3
node-size: 8gb
ttl: 240
```

Preferences file must be named as `.terrafarm` and placed in your `HOME` directory.

Command-line arguments have higher priority and overwrite properties defined in preferences file.

#### Usage

```
Usage: terrafarm <command> <options>

Commands:

  create      Create and run farm droplets on DigitalOcean
  destroy     Destroy farm droplets on DigitalOcean
  status      Show current Terrafarm preferencies and status

Options:

  --ttl, -t ttl           Max farm TTL (Time To Live)
  --output, -o file       Path to output file with access credentials
  --token, -T token       DigitalOcean token
  --key, -K key-file      Path to private key
  --region, -R region     DigitalOcean region
  --node-size, -N size    Droplet size on DigitalOcean
  --user, -U username     Build node user name
  --force, -f             Force command execution
  --no-color, -nc         Disable colors in output
  --help, -h              Show this help message
  --version, -v           Show version

Examples:

  terrafarm create --node-size 8gb --ttl 3h
  Create farm with redefined node size and TTL

  terrafarm create --force
  Forced farm creation (without prompt)

  terrafarm destroy
  Destory all farm nodes

  terrafarm status
  Show info about terrafarm

```

#### Build Status

| Repository | Status |
|------------|--------|
| Stable | [![Build Status](https://travis-ci.org/essentialkaos/terrafarm.svg?branch=master)](https://travis-ci.org/essentialkaos/terrafarm) |
| Unstable | [![Build Status](https://travis-ci.org/essentialkaos/terrafarm.svg?branch=develop)](https://travis-ci.org/essentialkaos/terrafarm) |

#### License

[EKOL](https://essentialkaos.com/ekol)
