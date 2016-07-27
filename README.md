# cred-alert

> scans repos for credentials and then shouts if it finds them

## Dependencies

Cred-alert depends on libmagic being installed on the system in order
to run. For building you'll need the libmagic header as well, usually
provided by a development package of the library.

On Mac OS

  $ brew install libmagic

On debian flavoured linux:

  $ apt-get install -y libmagic-dev

## Set Up

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

## Workflow

You can `go get` and edit the files like normal in this repository. If any
dependencies have changed them make sure to run `scripts/sync-submodules` in
order to make sure that the submodules are updated correctly.

You can generate a pretty commit message by running `scripts/commit-with-log`.

## CLI

### Building

The command line application can be built with the following command. Your
`$GOPATH` should already be set correctly by `direnv`.

    $ go build cred-alert/cmd/cred-alert-cli

### Examples

The default behavior of the cli is to read from standard input, scan for secrets, and report any
matches on standard output. It can also be used to recursively scan files in a directory.
Use --help to see all options.

#### Scan a directory


    $ ./cred-alert-cli -d src/cred-alert/


#### Scan from standard input


    $ cat src/cred-alert/sniff/patterns/samples_for_test.go | ./cred-alert-cli


## Server

The server app sets up an endpoint at `/webhook` to receive Github webhooks.
When it receives a [PushEvent][push-event], it will log any violations it
detects. Furthermore, if the Datadog environment variables are set, it will
count the violations in Datadog.

[push-event]: https://developer.github.com/v3/activity/events/types/#pushevent

### Building

The server can be built with the following command. Your `$GOPATH` should
already be set correctly by `direnv`.

    $ go build cred-alert/cmd/cred-alert

### Pushing to CF

#### Building with Concourse

We can use Concourse to build an application package that is identical to the
one that we build in CI.

    $ fly -t ci execute -x -c ci/build-app.yml -o cred-alert-app=/tmp/app

#### Deploying the Application

    $ cf push cred-alert -b binary_buildpack -p /tmp/app

When you push the application for the first time it will fail since the
necessary environment variables are not set and MySQL is not available.

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

### Environment Variables

| Name                        | Description                                                                      |
| --------------------------- | -------------------------------------------------------------------------------- |
| `DATA_DOG_ENVIRONMENT_TAG`  | Tag to use in emitted events (eg. `production`, `staging`)                       |
| `DATA_DOG_API_KEY`          | API key to use for Data Dog API access                                           |
| `GITHUB_WEBHOOK_SECRET_KEY` | Shared secret configured on github webhooks                                      |
| `PORT`                      | Port on which to listen for webhook requests for (set automatically if using CF) |
| `IGNORED_REPOS`             | A comma-separated list of patterns for repos to ignore (eg. `.*-credentials$`)   |
| `GITHUB_WEBHOOK_SECRET_KEY` | Shared secret configured on github webhooks                                      |
| `AWS_ACCESS_KEY`            | access key for aws SQS service                                                   |
| `AWS_SECRET_ACCESS_KEY`     | secret access key for aws SQS service                                            |
| `AWS_REGION`                | aws region for SQS service                                                       |
| `SQS_QUEUE_NAME`            | queue name to use with SQS                                                       |
