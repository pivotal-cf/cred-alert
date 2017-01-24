package main

import (
	"context"
	"cred-alert/config"
	"cred-alert/revokpb"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"regexp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	flags "github.com/jessevdk/go-flags"
)

type Opts struct {
	RPCServerAddress string `long:"rpc-server-address" description:"Address for RPC server." required:"true"`
	RPCServerPort    uint16 `long:"rpc-server-port" description:"Port for RPC server." required:"true"`

	CACertPath          string `long:"ca-cert-path" description:"Path to the CA certificate" required:"true"`
	ClientCertPath      string `long:"client-cert-path" description:"Path to the client certificate" required:"true"`
	ClientKeyPath       string `long:"client-key-path" description:"Path to the client private key" required:"true"`
	ClientKeyPassphrase string `long:"client-key-passphrase" description:"Passphrase for the client private key, if encrypted"`

	Query string `long:"query" short:"q" description:"Regular expression to search for" required:"true"`
}

func main() {
	var opts Opts

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		os.Exit(1)
	}

	serverAddr := fmt.Sprintf("%s:%d", opts.RPCServerAddress, opts.RPCServerPort)

	clientCert, err := config.LoadCertificate(
		opts.ClientCertPath,
		opts.ClientKeyPath,
		opts.ClientKeyPassphrase,
	)
	if err != nil {
		log.Fatalln(err)
	}

	rootCertPool, err := config.LoadCertificatePool(opts.CACertPath)
	if err != nil {
		log.Fatalln(err)
	}

	transportCreds := credentials.NewTLS(&tls.Config{
		ServerName:   opts.RPCServerAddress,
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      rootCertPool,
	})

	dialOption := grpc.WithTransportCredentials(transportCreds)
	conn, err := grpc.Dial(serverAddr, dialOption)
	if err != nil {
		log.Fatalf("failed to create handler: %s", err.Error())
	}
	defer conn.Close()

	revokClient := revokpb.NewRevokClient(conn)

	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			fmt.Print("cancelling search... ")
			cancel()
			fmt.Println("done")
			os.Exit(127)
		}
	}()

	stream, err := revokClient.Search(ctx, &revokpb.SearchQuery{
		Regex: opts.Query,
	})
	if err != nil {
		log.Fatalln(err.Error())
	}

	for {
		result, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatalln(err.Error())
		}

		show(result)
	}
}

var unsafeChars = regexp.MustCompile(`[[:^print:]]`)

func show(result *revokpb.SearchResult) {
	location := result.GetLocation()
	repo := location.GetRepository()

	content := result.GetContent()
	safe := unsafeChars.ReplaceAllLiteral(content, []byte{})

	fmt.Printf("[%s/%s@%s] %s:%d: %s\n", repo.GetOwner(), repo.GetName(), location.GetRevision()[:7], location.GetPath(), location.GetLineNumber(), safe)
}
