package main

import (
	"context"
	"fmt"
	"github.com/lightningnetwork/lnd/lnrpc"
	"math/rand"
	"os/exec"
	"sort"
	"time"
)

var connections map[string][]string
var peerinfo map[string]*lnrpc.GetInfoResponse

type Launcher struct {
	aliases   map[string]*alias
	nChannels int
}

func NewLauncher(aliases map[string]*alias, chans int) *Launcher {
	return &Launcher{
		aliases:   aliases,
		nChannels: chans,
	}
}

func (l *Launcher) createWallets() {
	for _, v := range l.aliases {
		logger.log("creating wallet: " + *v.Name)
		ln := unlocker(v)

		ctx := context.Background()
		seed, err := ln.GenSeed(ctx, &lnrpc.GenSeedRequest{})
		_, err = ln.InitWallet(ctx, &lnrpc.InitWalletRequest{
			WalletPassword:     []byte("password"),
			CipherSeedMnemonic: seed.CipherSeedMnemonic})
		if err != nil {
			logger.logerr("Cannot get info from node", err.Error())
			return
		}
		time.Sleep(3 * time.Second)
	}
}

func (l *Launcher) launchLnd() {
	u := 1
	for range l.aliases {
		cmd := exec.Command("lnd", fmt.Sprintf("--configfile=%s/.lndev/user%d/lnd.conf", userdir, u))

		err := cmd.Start()

		if err != nil {
			logger.logerr("lnd launch failure", err.Error())
		}
		time.Sleep(500 * time.Millisecond)

		u++
	}
}

func (l *Launcher) openChannels() {
	for _, a := range l.aliases {
		rpc := grpcClient(a)
		ctx := context.Background()
		for _, c := range connections[*a.Name] {

			logger.log(fmt.Sprintf("opening channel: %s -> %s", *a.Name, c))

			peer := peerinfo[c]
			_, err := rpc.OpenChannelSync(ctx, &lnrpc.OpenChannelRequest{
				NodePubkeyString:   peer.GetIdentityPubkey(),
				LocalFundingAmount: int64(rand.Intn(50000) + 100000),
			})
			if err != nil {
				logger.log(fmt.Sprintf("Cannot fund with peer %s\n\n", err))
				continue
			}
			l.generate(10)
			time.Sleep(2 * time.Second)

		}
	}
}

func (l *Launcher) fundNodes() {
	for _, a := range l.aliases {
		rpc := grpcClient(a)
		ctx := context.Background()

		addr, err := rpc.NewAddress(ctx, &lnrpc.NewAddressRequest{
			Type: lnrpc.AddressType_NESTED_PUBKEY_HASH,
		})
		if err != nil {
			logger.logerr("fund node address failure", err.Error())
			continue
		}
		cmd := exec.Command("bitcoin-cli", fmt.Sprintf("-conf=%s/.lndev/bitcoin/bitcoin.conf", userdir), "sendtoaddress", addr.Address, "1")
		err = cmd.Run()

		if err != nil {
			logger.logerr("fund node send failure", err.Error())
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
		aliaskeys = append(aliaskeys, key)
		connections[key] = []string{}
	}

	rand.Seed(time.Now().UnixNano())

	for _, v := range l.aliases {
		n := rand.Intn(l.nChannels) + 1

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
			logger.log(fmt.Sprintf("attempting connection: %s -> %s", *src.Name, *dest.Name))

			destrpc := grpcClient(dest)

			ctx := context.Background()
			destInfoResp, err := destrpc.GetInfo(ctx, &lnrpc.GetInfoRequest{})
			if err != nil {
				logger.logerr("destination get info failed", err.Error())
				return
			}

			srcrpc := grpcClient(src)
			_, err = srcrpc.ConnectPeer(ctx, &lnrpc.ConnectPeerRequest{
				Addr: &lnrpc.LightningAddress{
					Pubkey: destInfoResp.IdentityPubkey,
					Host:   fmt.Sprintf("127.0.0.1:%d", dest.Port+1000)},
				Perm: false})
			if err != nil {
				logger.logerr("source connect failure", err.Error())
				return
			}
			connections[*src.Name] = append(connections[*src.Name], *dest.Name)
			peerinfo[*dest.Name] = destInfoResp
			logger.log(fmt.Sprintf("[green]connected:[white] %s -> %s", *src.Name, *dest.Name))
			l.generate(1) // force chain sync?
			time.Sleep(1200 * time.Millisecond)
		}
	}

}

func (l *Launcher) launchNodes() {
	logger.log("launching bitcoin node")

	cmd := exec.Command("bitcoind", fmt.Sprintf("-conf=%s//.lndev/bitcoin/bitcoin.conf", userdir))

	err := cmd.Start()

	if err != nil {
		logger.logerr("bitcoin start fail", err.Error())
	}
	time.Sleep(2 * time.Second)
	l.generate(120)
	time.Sleep(1 * time.Second)

	logger.log("launching lnd nodes")
	l.launchLnd()

	l.generate(10) // syncs with chain
	l.createWallets()

	l.connectPeers()

	time.Sleep(2000 * time.Millisecond)

	logger.log("funding nodes")
	l.fundNodes()

	l.openChannels()
	logger.log("\n[green]Launch complete[white]\n")
	logger.done <- 0

}

func (l *Launcher) generate(n int) {
	out, err := exec.Command("bitcoin-cli", fmt.Sprintf("-conf=%s//.lndev/bitcoin/bitcoin.conf", userdir), "getnewaddress").Output()
	if err != nil {
		logger.logerr("get new address fail", err.Error())
	}

	cmd := exec.Command("bitcoin-cli", fmt.Sprintf("-conf=%s//.lndev/bitcoin/bitcoin.conf", userdir), "generatetoaddress", fmt.Sprintf("%d", n), string(out))
	err = cmd.Run()

	if err != nil {
		logger.logerr("generat block failure", err.Error())
	}
}
