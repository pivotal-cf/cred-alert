# cred-alert

> scans repos for credentials and then shouts if it finds them

## Getting Help

Come visit us in [Slack](https://pivotal.slack.com/messages/pcf-sec-enablement/)!

## CLI

### Building

The command line application can be built with the following command. Your
`$GOPATH` should already be set correctly by `direnv`.

    $ go build cred-alert/cmd/cred-alert-cli

### Examples

The default behavior of the cli is to read from standard input, scan for secrets, and report any
matches on standard output. It can also be used to recursively scan files in a directory.
Use --help to see all options.

#### Scan a file

    $ ./cred-alert-cli scan -f src/cred-alert/product.zip

#### Scan from standard input

    $ cat src/cred-alert/sniff/patterns/samples_for_test.go | ./cred-alert-cli scan

##### Scanning git diffs

Cred alert supports scanning diffs on standard input. When scanning a diff use the
`--diff` flag.

    $ git diff | ./cred-alert-cli scan --diff

#### Scan with custom RegExp

To override the default RegExp in order to scan for a specific vulnerability, use --regexp for a single RegExp or --regexp-file for newline delimited RegExp file

    $ git diff | ./cred-alert-cli scan --diff --regexp-file custom-regexp

#### Exit status

  `0` No error occurred and no credentials found

  `1` Miscellaneous error occurred

  `3` Found credentials

## Development

You'll need to install `gosub` in order to manage the submodules of this
project. It can be installed by running the following command (try to install
this in an outer $GOPATH so that you do not clutter up this directory with the
tooling):

    $ go get github.com/vito/gosub

In order to have your $GOPATH and $PATH set up properly when you enter this
directory you should install `direnv`. On macOS you can install this by running
this command and following the instructions to set up your shell:

    $ brew install direnv

The tests can be run using the `ginkgo` command line tool. This can be
installed with:

    $ go install github.com/onsi/ginkgo/ginkgo

The fakes can be generated using the `counterfeiter` tool. This can be
installed with:

    $ go get github.com/maxbrunsfeld/counterfeiter

You can `go get` and edit the files like normal in this repository. If any
dependencies have changed them make sure to run `scripts/sync-submodules` in
order to make sure that the submodules are updated correctly.

You can generate a pretty commit message by running `scripts/commit-with-log`.
