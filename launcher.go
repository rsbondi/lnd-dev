package main

import (
	"context"
	"fmt"
	"github.com/lightningnetwork/lnd/lnrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"os/exec"
	"os/user"
	"path"
	"time"
)

type Launcher struct {
	workingdir string
	aliases    map[string]*alias
}

func NewLauncher(wd string, aliases map[string]*alias) *Launcher {
	return &Launcher{
		workingdir: wd,
		aliases:    aliases,
	}
}

func unlocker(a *alias) lnrpc.WalletUnlockerClient {
	usr, err := user.Current()
	if err != nil {
		fmt.Println("Cannot get current user:", err)
		return nil
	}
	tlsCertPath := path.Join(usr.HomeDir, ".lnd/tls.cert")

	tlsCreds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		fmt.Println("Cannot get node tls credentials", err)
		return nil
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithBlock(),
	}

	host := fmt.Sprintf("localhost:%d", a.Port)
	conn, err := grpc.Dial(host, opts...)
	if err != nil {
		fmt.Println("cannot dial to lnd", err)
		return nil
	}
	client := lnrpc.NewWalletUnlockerClient(conn)

	return client
}

func (l *Launcher) launchNodes() {
	fmt.Fprintln(status, "launching bitcoin node")

	cmd := exec.Command("bitcoind", fmt.Sprintf("-conf=%s/bitcoin.conf", l.workingdir))

	err := cmd.Start()

	if err != nil {
		fmt.Fprintf(status, "%s\n", err.Error())
	}

	time.Sleep(2 * time.Second)
	u := 1
	for _, v := range l.aliases {
		if *v.Name == "Regtest" {
			continue
		}
		fmt.Fprintf(status, "launching node for %s\n with lnd --configfile=%s/profiles/user%d/lnd.conf command=%s\n", *v.Name, l.workingdir, u, *v.Path)
		cmd := exec.Command("lnd", fmt.Sprintf("--configfile=%s/profiles/user%d/lnd.conf", l.workingdir, u))

		err := cmd.Start()

		if err != nil {
			fmt.Fprintf(status, "%s\n", err.Error())
		}

		ln := unlocker(v)

		time.Sleep(200 * time.Millisecond)

		ctx := context.Background()
		seed, err := ln.GenSeed(ctx, &lnrpc.GenSeedRequest{})
		res, err := ln.InitWallet(ctx, &lnrpc.InitWalletRequest{
			WalletPassword:     []byte("password"),
			CipherSeedMnemonic: seed.CipherSeedMnemonic})
		if err != nil {
			fmt.Println("Cannot get info from node:", err)
			return
		}
		fmt.Fprintf(status, "%s\n", res)

		u++
	}
}
