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
	"sort"
	"time"
)

var connections map[string][]string
var peerinfo map[string]*lnrpc.GetInfoResponse

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
		logerr("current user fail", err.Error())
		return nil
	}
	tlsCertPath := path.Join(usr.HomeDir, ".lnd/tls.cert")
	macaroonPath := a.MacaroonPath

	tlsCreds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		logerr("cert failure", err.Error())
		return nil
	}

	macaroonBytes, err := ioutil.ReadFile(macaroonPath)
	if err != nil {
		logerr("macaroon file read failure", err.Error())
		return nil
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
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
		logerr("problem with grpc connection", err.Error())
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
	for _, v := range l.aliases {
		if *v.Name == "Regtest" {
			continue
		}

		ln := unlocker(v)

		ctx := context.Background()
		seed, err := ln.GenSeed(ctx, &lnrpc.GenSeedRequest{})
		_, err = ln.InitWallet(ctx, &lnrpc.InitWalletRequest{
			WalletPassword:     []byte("password"),
			CipherSeedMnemonic: seed.CipherSeedMnemonic})
		if err != nil {
			logerr("Cannot get info from node", err.Error())
			return
		}
		time.Sleep(3 * time.Second)
	}
}

func (l *Launcher) launchLnd() {
	u := 1
	for _, v := range l.aliases {
		if *v.Name == "Regtest" {
			continue
		}
		cmd := exec.Command("lnd", fmt.Sprintf("--configfile=%s/profiles/user%d/lnd.conf", l.workingdir, u))

		err := cmd.Start()

		if err != nil {
			logerr("lnd launch failure", err.Error())
		}

		u++
	}
}

func (l *Launcher) openChannels() {
	for _, a := range l.aliases {
		if *a.Name == "Regtest" {
			continue
		}
		rpc := grpcClient(a)
		ctx := context.Background()
		for _, c := range connections[*a.Name] {

			fmt.Fprintf(ui.cliresult, "opening channel: %s -> %s\n", *a.Name, c)
			app.Draw()

			peer := peerinfo[c]
			_, err := rpc.OpenChannelSync(ctx, &lnrpc.OpenChannelRequest{
				NodePubkeyString:   peer.GetIdentityPubkey(),
				LocalFundingAmount: int64(rand.Intn(50000) + 100000),
			})
			if err != nil {
				fmt.Fprintf(ui.cliresult, "Cannot fund with peer %s\n\n", err)
				app.Draw()
				continue
			}
			l.generate(10)
			time.Sleep(2 * time.Second)

		}
	}
}

func (l *Launcher) fundNodes() {
	for _, a := range l.aliases {
		if *a.Name == "Regtest" {
			continue
		}
		rpc := grpcClient(a)
		ctx := context.Background()

		addr, err := rpc.NewAddress(ctx, &lnrpc.NewAddressRequest{
			Type: lnrpc.AddressType_NESTED_PUBKEY_HASH,
		})
		if err != nil {
			logerr("fund node address failure", err.Error())
			continue
		}
		cmd := exec.Command("bitcoin-cli", fmt.Sprintf("-conf=%s/bitcoin.conf", l.workingdir), "sendtoaddress", addr.Address, "1")
		err = cmd.Run()

		if err != nil {
			logerr("fund node send failure", err.Error())
		}
	}
	l.generate(10)
	time.Sleep(2 * time.Second)
}

func (l *Launcher) connectPeers() {
	aliaskeys := make([]string, 0, len(l.aliases))

	connections = make(map[string][]string)
	peerinfo = make(map[string]*lnrpc.GetInfoResponse)
	for key := range l.aliases {
		if key != "Regtest" {
			aliaskeys = append(aliaskeys, key)
			connections[key] = []string{}
		}
	}

	rand.Seed(time.Now().UnixNano())

	for _, v := range l.aliases {
		if *v.Name == "Regtest" {
			continue
		}

		n := l.nChannels // rand.Intn(l.nChannels) + 1 // at least one

		for c := 0; c < n; c++ {
			src := v
			var dest *alias
			i := 0
			index := rand.Intn(len(aliaskeys))
			for {
				dest = l.aliases[aliaskeys[(index+i)%len(aliaskeys)]]
				if dest.Name != src.Name && sort.SearchStrings(connections[*src.Name], *dest.Name) == len(connections[*src.Name]) && sort.SearchStrings(connections[*dest.Name], *src.Name) == len(connections[*dest.Name]) {
					break
				}
				i++
				if i > n+len(aliaskeys) { // too many tries
					break
				}
			}
			if i > n+len(aliaskeys) {
				continue
			}
			fmt.Fprintf(ui.cliresult, "attempting connection: %s -> %s\n", *src.Name, *dest.Name)

			destrpc := grpcClient(dest)

			ctx := context.Background()
			destInfoResp, err := destrpc.GetInfo(ctx, &lnrpc.GetInfoRequest{})
			if err != nil {
				logerr("destination get info failed", err.Error())
				return
			}

			srcrpc := grpcClient(src)
			_, err = srcrpc.ConnectPeer(ctx, &lnrpc.ConnectPeerRequest{
				Addr: &lnrpc.LightningAddress{
					Pubkey: destInfoResp.IdentityPubkey,
					Host:   fmt.Sprintf("127.0.0.1:%d", dest.Port+1000)},
				Perm: false})
			if err != nil {
				logerr("source connect failure", err.Error())
				return
			}
			connections[*src.Name] = append(connections[*src.Name], *dest.Name)
			peerinfo[*dest.Name] = destInfoResp
			fmt.Fprintf(ui.cliresult, "connected!: %s -> %s\n", *src.Name, *dest.Name)
		}
	}

}

func log(s string) {
	fmt.Fprintln(ui.cliresult, s)
	app.Draw()
}

func logerr(s string, e string) {
	msg := fmt.Sprintf("[red]%s: [white]%s", s, e)
	fmt.Fprintln(ui.cliresult, msg)
	app.Draw()
}

func (l *Launcher) launchNodes() {
	log("launching bitcoin node")

	cmd := exec.Command("bitcoind", fmt.Sprintf("-conf=%s/bitcoin.conf", l.workingdir))

	err := cmd.Start()

	if err != nil {
		logerr("bitcoin start fail", err.Error())
	}
	time.Sleep(2 * time.Second)

	log("launching lnd")
	l.launchLnd()

	l.generate(10) // syncs with chain
	log("creating wallets")
	l.createWallets()

	log("connecting peers")
	l.connectPeers()

	time.Sleep(3200 * time.Millisecond)

	log("funding nodes")
	l.fundNodes()

	log("opening channels")
	l.openChannels()
	log("[green]Launch complete[white]")

}

func (l *Launcher) generate(n int) {
	out, err := exec.Command("bitcoin-cli", fmt.Sprintf("-conf=%s/bitcoin.conf", l.workingdir), "getnewaddress").Output()
	if err != nil {
		logerr("get new address fail", err.Error())
	}

	cmd := exec.Command("bitcoin-cli", fmt.Sprintf("-conf=%s/bitcoin.conf", l.workingdir), "generatetoaddress", fmt.Sprintf("%d", n), string(out))
	err = cmd.Run()

	if err != nil {
		logerr("generat block failure", err.Error())
	}
}
