package main

import (
	"context"
	"fmt"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
	"io/ioutil"
	"math/rand"
	"os/exec"
	"os/user"
	"path"
	"time"
)

type Launcher struct {
	workingdir string
	aliases    map[string]*alias
	nChannels  int
}

func NewLauncher(wd string, aliases map[string]*alias, chans int) *Launcher {
	return &Launcher{
		workingdir: wd,
		aliases:    aliases,
		nChannels:  chans,
	}
}

func grpcClient(a *alias) lnrpc.LightningClient {
	usr, err := user.Current()
	if err != nil {
		fmt.Fprintf(status, "Cannot get current user: %s\n", err)
		return nil
	}
	tlsCertPath := path.Join(usr.HomeDir, ".lnd/tls.cert")
	macaroonPath := a.MacaroonPath
	// /home/bondibit/projects/lnd-dev/profiles/user1/data/chain/bitcoin/regtest/admin.macaroon

	tlsCreds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		fmt.Fprintf(status, "Cannot get node tls credentials %s\n", err)
		return nil
	}

	macaroonBytes, err := ioutil.ReadFile(macaroonPath)
	if err != nil {
		fmt.Fprintf(status, "Cannot read macaroon file %s\n", err)
		return nil
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
		fmt.Fprintf(status, "Cannot unmarshal macaroon %s\n", err)
		return nil
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)),
	}

	host := fmt.Sprintf("localhost:%d", a.Port)
	conn, err := grpc.Dial(host, opts...)
	if err != nil {
		fmt.Fprintf(status, "cannot dial to lnd %s\n", err)
		return nil
	}
	client := lnrpc.NewLightningClient(conn)
	return client
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

func (l *Launcher) createWallets() {
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

func (l *Launcher) createChannels() {
	aliaskeys := make([]string, 0, len(l.aliases))

	for key := range l.aliases {
		if key != "Regtest" {
			aliaskeys = append(aliaskeys, key)
		}
	}

	fmt.Fprintf(status, "keys for aliases: %s\n", aliaskeys)
	rand.Seed(time.Now().UnixNano())

	for _, v := range l.aliases {
		if *v.Name == "Regtest" {
			continue
		}

		n := rand.Intn(l.nChannels)
		n = 1 // temp hack to test
		fmt.Fprintf(status, "random channels: %d of %d\n", n, l.nChannels)
		for c := 0; c < n; c++ {
			src := l.aliases[aliaskeys[0]] //v
			var dest *alias
			dest = l.aliases[aliaskeys[1]]
			// for {
			// 	rand.Seed(time.Now().UnixNano())
			// 	dest = l.aliases[aliaskeys[rand.Intn(len(aliaskeys))]]
			// 	if dest.Name != src.Name {
			// 		break
			// 	}
			// }

			destrpc := grpcClient(dest)

			ctx := context.Background()
			destInfoResp, err := destrpc.GetInfo(ctx, &lnrpc.GetInfoRequest{})
			if err != nil {
				fmt.Fprintf(status, "Cannot get info from node: %s", err)
				return
			}

			srcrpc := grpcClient(src)
			// srcInfoResp, err := srcrpc.GetInfo(ctx, &lnrpc.GetInfoRequest{})
			// if err != nil {
			// 	fmt.Fprintf(status, "Cannot get info from node: %s", err)
			// 	return
			// }
			connectResponse, err := srcrpc.ConnectPeer(ctx, &lnrpc.ConnectPeerRequest{
				Addr: &lnrpc.LightningAddress{
					Pubkey: destInfoResp.IdentityPubkey,
					Host:   fmt.Sprintf("127.0.0.1:%d", dest.Port+1000)},
				Perm: false})
			if err != nil {
				fmt.Fprintf(status, "Cannot connect to peer %s", err)
				return
			}
			fmt.Fprintf(status, "%q\n", connectResponse)
		}
		break
	}
}

func (l *Launcher) launchNodes() {
	fmt.Fprintln(status, "launching bitcoin node")

	cmd := exec.Command("bitcoind", fmt.Sprintf("-conf=%s/bitcoin.conf", l.workingdir))

	err := cmd.Start()

	if err != nil {
		fmt.Fprintf(status, "%s\n", err.Error())
	}
	time.Sleep(2 * time.Second)

	l.createWallets()
	time.Sleep(5 * time.Second)
	l.generate()
	time.Sleep(2 * time.Second)
	l.createChannels()
}

func (l *Launcher) generate() {
	out, err := exec.Command("bitcoin-cli", fmt.Sprintf("-conf=%s/bitcoin.conf", l.workingdir), "getnewaddress").Output()
	if err != nil {
		fmt.Fprintf(status, "%s\n", err.Error())
	}

	cmd := exec.Command("bitcoin-cli", fmt.Sprintf("-conf=%s/bitcoin.conf", l.workingdir), "generatetoaddress", "10", string(out))
	err = cmd.Run()

	if err != nil {
		fmt.Fprintf(status, "%s\n", err.Error())
	}
}
