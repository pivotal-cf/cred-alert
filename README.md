# cred-alert

> scans repos for credentials and then shouts if it finds them

## CLI
### Installing

Pre-built versions of the `cred-alert-cli` binary are available for download. View the [latest
release on GitHub](https://github.com/pivotal-cf/cred-alert/releases/latest) and download either
`cred-alert-cli_darwin` (for macOS) or `cred-alert-cli_linux` (for Linux). Simply save it in any
directory on your `PATH` and make it executable.

### Examples

The default behavior of the cli is to read from standard input, scan for secrets, and report any
matches on standard output. It can also be used to recursively scan files in a directory.
Use `--help` to see all options.

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

To override the default RegExp in order to scan for a specific vulnerability, use `--regexp`
for a single RegExp or `--regexp-file` for a newline-delimited RegExp file.

    $ git diff | ./cred-alert scan --diff --regexp-file custom-regexp

#### Exit status

  `0` No error occurred and no credentials found

  `1` Miscellaneous error occurred

  `3` Found credentials

## Development

To run the tests:

    go test ./...

To build the CLI:

    go build -x -v -o cred-alert-cli
