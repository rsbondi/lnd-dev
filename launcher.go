package main

import (
	"fmt"
	"os/exec"
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

		time.Sleep(200 * time.Millisecond)

		// TODO: I think there is a subprocess issue, probable create with grpc
		// cmd = v.Command("create")
		// stdin, err := cmd.StdinPipe()

		// cmd.Start()

		// stdin.Write([]byte("password\n"))
		// stdin.Write([]byte("password\n"))
		// stdin.Write([]byte("n\n"))
		// stdin.Write([]byte("\n"))

		/*
		   create
		   Input wallet password:
		   Confirm wallet password:

		   Do you have an existing cipher seed mnemonic you want to use? (Enter y/n): n

		   Your cipher seed can optionally be encrypted.
		   Input your passphrase if you wish to encrypt it (or press enter to proceed without a cipher seed passphrase):
		*/
		u++
	}
}
