                             _             _           _
            ___ _ __ ___  __| |       __ _| | ___ _ __| |_
           / __| '__/ _ \/ _` |_____ / _` | |/ _ \ '__| __|
          | (__| | |  __/ (_| |_____| (_| | |  __/ |  | |_
           \___|_|  \___|\__,_|      \__,_|_|\___|_|   \__|

     scans repos for credentials and then shouts if it finds them


## set up

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


## workflow

You can `go get` and edit the files like normal in this repository. If any
dependencies have changed them make sure to run `scripts/sync-submodules` in
order to make sure that the submodules are updated correctly.

You can generate a pretty commit message by running `scripts/commit-with-log`.

# Command Line App

## Building

```
export GOPATH=$WORKSPACE/cred-alert
go build cred-alert/cmd/cred-alert-cli
```

# Server

The server app will set up an endpoint at `/webhook` to receive Github webhooks. When it receives a [PushEvent](https://developer.github.com/v3/activity/events/types/#pushevent), it will log any violations it detects. Furthermore, if the Datadog environment variables are set, it will count the violations in Datadog.

## Building

```
export GOPATH=$WORKSPACE/cred-alert
go build cred-alert/cmd/server
```

## Pushing to CF

To use with CF, build the app into a linux binary, then push to CF using the binary buildpack. In this example we'll use docker to build our linux binary.

##### Create docker image
```
cd $WORKSPACE/cred-alert
docker run -v $(pwd):/app -it cloudfoundry/cflinuxfs2 bash
```
##### Inside docker
```
apt-get install golang -y
export GOPATH=/app
cd /app
go build cred-alert/cmd/server
exit
```
##### Outside docker
```
cf push cred-alert -c './server' -b binary_buildpack
```

When you push the app the first time, it will fail since the necessary environment variables are not set. Referr to the next section for all the needed environment variables.

## Environment Variables

| Name | Description |
| ---- | ----------- |
| `DATA_DOG_ENVIRONMENT_TAG` | Tag to use in emitted events (eg. `production`, `staging`) |
| `DATA_DOG_API_KEY` | API key to use for Data Dog api access |
| `GITHUB_WEBHOOK_SECRET_KEY` | shared secret configured on github webhooks and|
| `PORT` | port on which to listen for webhook requests for (set automatically if using CF) |

