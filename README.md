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

## Server

The server app sets up an endpoint at `/webhook` to receive Github webhooks.
When it receives a [PushEvent][push-event], it will log any violations it
detects. Furthermore, if the Datadog environment variables are set, it will
count the violations in Datadog.

[push-event]: https://developer.github.com/v3/activity/events/types/#pushevent

### Building

The server has two components which need to be built: the ingestor and the worker. They can be built with the following commands. Your `$GOPATH` should
already be set correctly by `direnv`.

    $ go build cred-alert/cmd/cred-alert-ingestor
    $ go build cred-alert/cmd/cred-alert-worker

### Pushing to CF

#### Building with Concourse

We can use Concourse to build an application package that is identical to the
one that we build in CI.

    $ fly -t ci execute -x -c ci/compile-components.yml -o cred-alert-components=/tmp/app

#### Deploying the Application

    $ cf push cred-alert-ingestor -b binary_buildpack -p /tmp/app -c ./cred-alert-ingestor --no-start
    $ cf push cred-alert-worker -b binary_buildpack -p /tmp/app -c ./cred-alert-worker --no-start

Before you can run the application you'll need to bind `cred-alert-worker` to a mysql service
and set necessary environment variables on both apps.

#### Binding MySQL

Before the worker will start, the app needs to be bound to a MySQL service instance named `cred-alert-mysql`.

For local development the worker can be started with the following command line parameters:

| Parameter                 | Description         |
| ------------------------- | ------------------- |
| --mysql-username=USERNAME | MySQL username      |
| --mysql-password=PASSWORD | MySQL password      |
| --mysql-hostname=HOSTNAME | MySQL hostname      |
| --mysql-port=PORT         | MySQL port          |
| --mysql-dbname=DBNAME     | MySQL database name |

#### Environment Variables

##### Common to both Ingestor and Worker

| Name                        | Description                                                                      |
| --------------------------- | -------------------------------------------------------------------------------- |
| `ENVIRONMENT`               | Tag to use in emitted events (eg. `production`, `staging`)                       |
| `DATA_DOG_API_KEY`          | API key to use for Datadog API access                                            |
| `AWS_ACCESS_KEY`            | Access key for AWS SQS service                                                   |
| `AWS_SECRET_ACCESS_KEY`     | Secret access key for AWS SQS service                                            |
| `AWS_REGION`                | AWS region for SQS service                                                       |
| `SQS_QUEUE_NAME`            | Queue name to use with SQS                                                       |

##### Ingestor

| Name                        | Description                                                                      |
| --------------------------- | -------------------------------------------------------------------------------- |
| `GITHUB_WEBHOOK_SECRET_KEY` | Shared secret configured on Github webhooks                                      |
| `PORT`                      | Port on which to listen for webhook requests for (set automatically if using CF) |
| `IGNORED_REPOS`             | A comma-separated list of patterns for repos to ignore (eg. `.*-credentials$`)   |

##### Worker

| Name                        | Description                                                                      |
| --------------------------- | -------------------------------------------------------------------------------- |
| `GITHUB_ACCESS_TOKEN`       | Access token used to access the Github API                                       |
| `SLACK_WEBHOOK_URL`         | URL for sending Slack notifications                                              |

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
