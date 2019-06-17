package main

import (
	"os/exec"
	"sort"
	"strings"
)

type name struct {
	Last string `json:"last"`
}

type apiname struct {
	Name name `json:"name"`
}

type apiresults struct {
	Results []apiname `json:"results"`
}

type cfgview struct {
	N    int
	Name string
}

const configtemplate = `[Application Options]
datadir=profiles/user{{.N}}/data
logdir=profiles/user{{.N}}/log
debuglevel=info
debughtlc=true
rpclisten=localhost:1000{{.N}}
listen=localhost:1001{{.N}}
restlisten=localhost:800{{.N}}
alias={{.Name}}

[Bitcoin]
bitcoin.regtest=1
bitcoin.active=1
bitcoin.node=bitcoind

[Bitcoind]
bitcoind.rpcuser=kek
bitcoind.rpcpass=kek
bitcoind.zmqpubrawblock=tcp://127.0.0.1:28332
bitcoind.zmqpubrawtx=tcp://127.0.0.1:28333
`

type node struct {
	Buff     string
	Cmds     []string
	CmdIndex *int
}

type alias struct {
	Name *string
	Path *string
}

func (a *alias) Command(c ...string) *exec.Cmd {
	clicmd := strings.Split(*a.Path, " ")
	cliarg := clicmd[1:]
	cliargs := append(cliarg, c...)
	cmd := exec.Command(clicmd[0], cliargs...)
	return cmd

}

func sortAliasKeys(a map[string]*alias) []string {
	keys := make([]string, 0, len(a))

	for key := range a {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys
}

const BASE_PORT = 10000
