<p align="center"><a href="#usage-demo">Usage demo</a> • <a href="#installation">Installation</a> • <a href="#configuration">Configuration</a> • <a href="#debugging">Debugging</a> • <a href="#usage">Usage</a> • <a href="#build-status">Build Status</a> • <a href="#contributing">Contributing</a> • <a href="#license">License</a></p>

<p align="center">
<img width="200" height="200" src="https://essentialkaos.com/github/terrafarm.png"/>
</p>

`terrafarm` is utility for working with [Terraform](https://www.terraform.io)-based [rpmbuilder](https://github.com/essentialkaos/rpmbuilder) farm on [DigitalOcean](https://www.digitalocean.com).

## Usage demo

[![asciicast](https://essentialkaos.com/github/terrafarm-0101.gif)](https://asciinema.org/a/85405)

## Installation

To build the terrafarm from scratch, make sure you have a working Go 1.5+ workspace ([instructions](https://golang.org/doc/install)) and latest version of [Terraform](https://www.terraform.io/downloads.html), then:

```
go get github.com/essentialkaos/terrafarm
```

If you want update terrafarm to latest stable release, do:

```
go get -u github.com/essentialkaos/terrafarm
```

## Configuration

`terrafarm` have three ways for farm configuration — preferences file, environment variables and command-line arguments.

You can use all three ways simultaneously, but in this case `terrafarm` uses different priority for each way:

1. Preferences file (_lowest priority_)
2. Environment variables
3. Command-line arguments (_highest priority_)

### Preferences file

Preferences file use next format:

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
ttl: 2h
```

Preferences file must be named as `.terrafarm` and placed in your `HOME` directory.

### Environment variables

_Environment variables overwrite properties defined in preferences file._

You can define or redefine properties using next variables:

* `TERRAFARM_DATA` - Path to directory with your own Terraform data
* `TERRAFARM_TTL` - Max farm TTL (Time To Live)
* `TERRAFARM_MAX_WAIT` - Max time which monitor will wait if farm have active build
* `TERRAFARM_OUTPUT` - Path to output file with access credentials
* `TERRAFARM_TEMPLATE` - Farm template name
* `TERRAFARM_TOKEN` - DigitalOcean token
* `TERRAFARM_KEY` - Droplet size on DigitalOcean
* `TERRAFARM_REGION` - DigitalOcean region
* `TERRAFARM_NODE_SIZE` - Droplet size on DigitalOcean
* `TERRAFARM_USER` - Build node user name
* `TERRAFARM_PASSWORD` - Build node user password

Example:

```bash
TERRAFARM_DATA=/home/user/my-own-terraform-data TERRAFARM_TTL=1h terrafarm create
```

### Command-line arguments

_Command-line arguments overwrite properties defined in preferences file and environment variables._

All supported command-line arguments with usage examples can be found in [usage](#usage) section.

## Debugging

If you find an bug with Terrafarm, please include the detailed log. As a user, this information can help work around the problem and prepare fixes. 

First of all, you should specify `-D` or `--debug` argument with Terrafarm to print the output of command which would be executed. It might be useful to know what exactly parameters would be passed to Terraform.

Also keep in mind that Terrafarm works with Terraform and you should know how to debug it. We recommend to use `DEBUG` or `TRACE` values to find possible problems with Terraform. This will cause detailed logs to appear on stderr. To persist logged output you can set `TF_LOG_PATH` to write the log to a specific file.

## Usage

```
Usage: terrafarm <command> <options>

Commands:

  create template-name    Create and run farm droplets on DigitalOcean
  destroy                 Destroy farm droplets on DigitalOcean
  status                  Show current Terrafarm preferences and status
  templates               List all available farm templates
  prolong ttl max-wait    Increase TTL or set max wait time

Options:

  --ttl, -t time             Max farm TTL (Time To Live)
  --max-wait, -w time        Max time which monitor will wait if farm have active build
  --output, -o file          Path to output file with access credentials
  --token, -T token          DigitalOcean token
  --key, -K key-file         Path to private key
  --region, -R region        DigitalOcean region
  --node-size, -N size       Droplet size on DigitalOcean
  --user, -U username        Build node user name
  --password, -P password    Build node user password
  --force, -f                Force command execution
  --no-validate, -nv         Don't validate preferences
  --no-color, -nc            Disable colors in output
  --help, -h                 Show this help message
  --version, -v              Show version

Examples:

  terrafarm create --node-size 8gb --ttl 3h
  Create farm with redefined node size and TTL

  terrafarm create --force
  Forced farm creation (without prompt)

  terrafarm create c6-multiarch-fast
  Create farm from template c6-multiarch-fast

  terrafarm destroy
  Destroy all farm nodes

  terrafarm status
  Show info about terrafarm

  terrafarm prolong 1h 15m
  Increase TTL on 1 hour and set max wait to 15 minutes

```

## Build Status

| Repository | Status |
|------------|--------|
| Stable | [![Build Status](https://travis-ci.org/essentialkaos/terrafarm.svg?branch=master)](https://travis-ci.org/essentialkaos/terrafarm) |
| Unstable | [![Build Status](https://travis-ci.org/essentialkaos/terrafarm.svg?branch=develop)](https://travis-ci.org/essentialkaos/terrafarm) |

## Contributing

Before contributing to this project please read our [Contributing Guidelines](https://github.com/essentialkaos/contributing-guidelines#contributing-guidelines).

## License

[EKOL](https://essentialkaos.com/ekol)
