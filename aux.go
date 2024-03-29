package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	N        int
	Rpc      int
	Rest     int
	Listen   int
	Name     string
	Macaroon string
	User     string
}

type Logger struct {
	out  chan string
	done chan int
}

func NewLogger(out chan string, done chan int) *Logger {
	return &Logger{
		out:  out,
		done: done,
	}
}

func (l *Logger) log(s string) {
	l.out <- s
}

func (l *Logger) logerr(s string, e string) {
	msg := fmt.Sprintf("[red]%s: [white]%s", s, e)
	l.out <- msg
}

var logger *Logger

const configtemplate = `[Application Options]
datadir={{.User}}/.lndev/user{{.N}}/data
logdir={{.User}}/.lndev/user{{.N}}/log
debuglevel=info
debughtlc=true
rpclisten=localhost:{{.Rpc}}
listen=localhost:{{.Listen}}
restlisten=localhost:{{.Rest}}
alias={{.Name}}
adminmacaroonpath={{.Macaroon}}

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

const bitcoinconf = `server=1
txindex=1
daemon=1
regtest=1
maxconnections=10
rpcuser=kek
rpcpassword=kek
minrelaytxfee=0.00000000
incrementalrelayfee=0.00000010
zmqpubrawblock=tcp://127.0.0.1:28332
zmqpubrawtx=tcp://127.0.0.1:28333
datadir={{.}}/.lndev/bitcoin
`

type node struct {
	Buff     string
	Cmds     []string
	CmdIndex *int
}

type alias struct {
	Name         *string
	Path         *string
	Port         int
	MacaroonPath string
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

func randomNames() *apiresults {
	url := fmt.Sprintf("https://randomuser.me/api/?results=%s&inc=name", nNodes)
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	decoder := json.NewDecoder(reader)
	names := &apiresults{}
	decoder.Decode(&names)
	return names
}

func ensureDir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}
	}
}

const BASE_PORT = 10000
