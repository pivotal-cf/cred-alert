# cred-alert

> scans repos for credentials and then shouts if it finds them

## CLI
### Installing

#### Downloading

Pre-built versions of the `cred-alert-cli` binary are available for download. To 
install download the correct version ([macOs][cred-alert-osx] or [Linux][cred-alert-linux]),
rename the file `cred-alert-cli`, make it executable, and move it to a directory in `${PATH}`.

```
os_name=$(uname | awk '{print tolower($1)}')
curl -o cred-alert-cli \
  https://s3.amazonaws.com/cred-alert/cli/current-release/cred-alert-cli_${os_name}
chmod 755 cred-alert-cli
mv cred-alert-cli /usr/local/bin # <= or other directory in ${PATH}
```

#### Building

The command line application can be built with the following command. Your
`$GOPATH` should already be set correctly by `direnv`.

    $ go install github.com/pivotal-cf/cred-alert

### Examples

The default behavior of the cli is to read from standard input, scan for secrets, and report any
matches on standard output. It can also be used to recursively scan files in a directory.
Use --help to see all options.

#### Scan a file

    $ ./cred-alert scan -f src/cred-alert/product.zip

#### Scan a directory

    $ ./cred-alert scan -f .

#### Scan from standard input

    $ cat sniff/patterns/samples_for_test.go | ./cred-alert scan

##### Scanning git diffs

Cred alert supports scanning diffs on standard input. When scanning a diff use the
`--diff` flag.

    $ git diff | ./cred-alert scan --diff

#### Scan with custom RegExp

To override the default RegExp in order to scan for a specific vulnerability, use --regexp for a single RegExp or --regexp-file for newline delimited RegExp file

    $ git diff | ./cred-alert scan --diff --regexp-file custom-regexp

#### Exit status

  `0` No error occurred and no credentials found

  `1` Miscellaneous error occurred

  `3` Found credentials

### Additional usage documentation

[Cred-Alert CLI Instructions - SIMPLE](https://sites.google.com/a/pivotal.io/cloud-foundry/process/security/cred-alert-cli-instructions)

## Development

The tests can be run using the `ginkgo` command line tool. This can be
installed with:

    $ go install github.com/onsi/ginkgo/ginkgo

The fakes can be generated using the `counterfeiter` tool. This can be
installed with:

    $ go get github.com/maxbrunsfeld/counterfeiter

You can generate a pretty commit message by running `scripts/commit-with-log`.

[cred-alert-osx]: https://s3.amazonaws.com/cred-alert/cli/current-release/cred-alert-cli_darwin
[cred-alert-linux]: https://s3.amazonaws.com/cred-alert/cli/current-release/cred-alert-cli_linux
