# cred-alert

> scans repos for credentials and then shouts if it finds them

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
necessary environment variables are not set. Refer to the next section for all
the needed environment variables.

### Environment Variables

| Name                        | Description                                                                      |
| --------------------------- | -------------------------------------------------------------------------------- |
| `DATA_DOG_ENVIRONMENT_TAG`  | Tag to use in emitted events (eg. `production`, `staging`)                       |
| `DATA_DOG_API_KEY`          | API key to use for Data Dog API access                                           |
| `GITHUB_WEBHOOK_SECRET_KEY` | Shared secret configured on github webhooks                                      |
| `PORT`                      | Port on which to listen for webhook requests for (set automatically if using CF) |
| `IGNORED_REPOS`             | A comma-separated list of patterns for repos to ignore (eg. `.*-credentials$`)   |

