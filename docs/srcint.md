## Introduction
srcint is a command line utility that performs text searches across all the orgs that Pivotal contributes to
including only those that we scan as part of our cred-alert infrastructure

In order to run `srcint` it is necessary to locate the `revok-worker` server 
within the `cred-alert` bosh deployment, provision that server with the 
`srcint` executable, and then run the `srcint` CLI with the search query.

### Building `srcint`
`git clone git@github.com:pivotal-cf/cred-alert`
`cd cred-alert && direnv allow` # sets up the go path
`cd src/cred-alert/cmd/srcint`
`go build`
After these steps you should see the `srcint` binary in the current directory

### Putting `srcint` on the `revok-worker` server
Copy it into the bastion server
`scp srcint <username>@bastion:/tmp/`
Ssh onto the bastion so that you can get access to bosh
`ssh <username>@bastion`
Then move the binary over to the VM that is running the `revok-worker`
`bosh -e bosh -d cred-alert scp /tmp/srcint revok:/tmp`

### Ssh-ing to the `revok-worker`
`bosh -e bosh -d cred-alert ssh revok`
`sudo -i` #runs a login shell
`cp /tmp/srcint .`


### Finding the `revok-worker` configuration
`less /var/vcap/jobs/revok/config/config.yml`
Take note of the following keys:
```
  identity.ca_certificate_path
  identity.certificate_path
  identity.private_key_path
  rpc_server.bind_port #usually 50051
```

### Running `srcint`
As the srcint client is now only run local to the revok-worker server, we need a dns mapping for that locally.

To obtain the Common-Name (CN) of the server run:
`openssl x509 -noout -text -in <server_certificate>`

Add a mapping to `/etc/hosts` file for that Common-Name to 127.0.0.1 e.g. `revok-worker 127.0.0.1`

Then use that dns name in the command below:

`./srcint --rpc-server-address <common-name> --rpc-server-port <rpc_server.bind_port> --ca-cert-path <identity.ca_certificate_path> --client-cert-path <identity.certificate_path> --client-key-path <identity.private_key_path> -q <your query>`
