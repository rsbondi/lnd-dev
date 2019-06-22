package main

import (
	"bytes"
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strings"
	"text/template"
)

type MainUI struct {
	cli         *tview.InputField
	list        *tview.DropDown
	cliresult   *tview.TextView
	currentnode string
	aliases     map[string]*alias
	nodes       map[string]*node
}

var userdir string

func NewMainUI() *MainUI {
	usr, err := user.Current()
	if err != nil {
		panic("current user fail: " + err.Error())
	}
	userdir = usr.HomeDir

	dir := path.Join(userdir, ".lndev")
	ensureDir(dir)
	ensureDir(path.Join(dir, "bitcoin"))

	ui := &MainUI{
		cliresult: tview.NewTextView().SetDynamicColors(true),
		cli:       tview.NewInputField(),
		list:      tview.NewDropDown(),
		aliases:   make(map[string]*alias),
		nodes:     make(map[string]*node),
	}
	ui.cliresult.SetBorder(false)

	ui.cliresult.SetInputCapture(func(key *tcell.EventKey) *tcell.EventKey {
		if key.Key() == tcell.KeyCtrlL {
			ui.cliresult.SetText("")
			ui.nodes[ui.currentnode].Buff = ""
		}
		return key
	})

	ui.cli.
		SetPlaceholder("Enter cli command - use Ctrl+v to paste (no shift)").
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetFieldWidth(0).SetBorder(true).SetTitle("CLI (Ctrl+i) for CLI (Ctrl+o) for results")

	ui.cli.SetInputCapture(ui.cliInputCapture)

	return ui

}

func (u *MainUI) cliInputCapture(key *tcell.EventKey) *tcell.EventKey {
	if key.Key() == tcell.KeyEnter {
		go (func() {
			cmdnode := u.currentnode
			text := u.cli.GetText()
			cmdfmt := fmt.Sprintf("[#00aaaa]# %s[white]\n", text)
			if text == "" {
				fmt.Fprintf(u.cliresult, "Please provide a command to execute\n")
			}
			args, err := parseCommandLine(text)
			if err != nil {
				fmt.Fprintf(u.cliresult, "%s\n", err.Error())
			}

			u.cli.SetText("")
			app.Draw()

			cmd := u.aliases[cmdnode].Command(args...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Fprintf(u.cliresult, "%s\n", err.Error())
			}

			if cmdnode == u.currentnode {
				fmt.Fprintf(u.cliresult, cmdfmt)
				fmt.Fprintf(u.cliresult, "%s\n", tview.Escape(string(out)))
				u.cliresult.ScrollToEnd()
				app.Draw()
			}
			u.nodes[cmdnode].Buff += cmdfmt
			u.nodes[cmdnode].Buff += string(out) + "\n"

			cmdsize := len(u.nodes[cmdnode].Cmds)
			if *u.nodes[cmdnode].CmdIndex == -1 || u.nodes[cmdnode].Cmds[cmdsize-1] != text {
				u.nodes[cmdnode].Cmds = append(u.nodes[cmdnode].Cmds, text)
			}
			*u.nodes[cmdnode].CmdIndex = len(u.nodes[cmdnode].Cmds)

		})()
	} else if key.Key() == tcell.KeyUp {
		index := u.nodes[u.currentnode].CmdIndex
		if *index > 0 {
			*index = *index - 1
		}
		if *index >= 0 && *index < len(u.nodes[u.currentnode].Cmds) {
			u.cli.SetText(u.nodes[u.currentnode].Cmds[*index])
		}
	} else if key.Key() == tcell.KeyDown {
		index := u.nodes[u.currentnode].CmdIndex
		if *index == len(u.nodes[u.currentnode].Cmds)-1 {
			u.cli.SetText("")
			*index = *index + 1
			return key
		}
		if *index < len(u.nodes[u.currentnode].Cmds)-1 {
			*index = *index + 1
		}
		if *index >= 0 && *index < len(u.nodes[u.currentnode].Cmds) {
			u.cli.SetText(u.nodes[u.currentnode].Cmds[*index])
		}
	} else if key.Key() == tcell.KeyCtrlV {
		clip, err := clipboard.ReadAll()
		if err != nil {
			fmt.Fprintf(u.cliresult, "%s\n", err.Error())
		} else {
			full := strings.Replace(clip, "\n", "", -1)
			u.cli.SetText(fmt.Sprintf("%s%s", u.cli.GetText(), full)) // TODO: this only paste to end, fix for insert
		}
	}
	return key

}

func (u *MainUI) populateList(r []apiname) {
	u.defineNodes(r)
	aliasKeys := sortAliasKeys(u.aliases)
	for _, a := range aliasKeys {
		s := -1
		anode := &node{"", []string{}, &s}
		u.nodes[*u.aliases[a].Name] = anode

		name := *u.aliases[a].Name
		u.list.AddOption(*u.aliases[a].Name, func() {
			u.cli.SetText("")
			u.currentnode = name
			u.cliresult.SetText(u.nodes[name].Buff)
			app.SetFocus(u.cli)
		})
	}

	confcmd := fmt.Sprintf("bitcoin-cli -conf=%s//.lndev/bitcoin/bitcoin.conf", userdir)
	name := "Regtest"
	u.aliases[name] = &alias{&name, &confcmd, 0, ""}
	s := -1
	anode := &node{"", []string{}, &s}
	u.nodes[name] = anode

	u.list.AddOption(name, func() {
		u.cli.SetText("")
		u.currentnode = name
		u.cliresult.SetText(u.nodes[name].Buff)
		app.SetFocus(u.cli)
	})
	u.list.AddOption("Quit", func() {
		// kill bitcoind
		cmd := exec.Command("bitcoin-cli", fmt.Sprintf("-conf=%s//.lndev/bitcoin/bitcoin.conf", userdir), "stop")
		cmd.Run()

		for i := 1; i < len(u.aliases); i++ {
			host := fmt.Sprintf("--rpcserver=localhost:%d", BASE_PORT+i)
			macaroon := fmt.Sprintf("--macaroonpath=%s/.lndev/user%d/data/chain/bitcoin/regtest/admin.macaroon", userdir, i)
			cmd := exec.Command("lncli", host, macaroon, "stop")
			cmd.Run()
		}

		os.RemoveAll(fmt.Sprintf("%s/.lndev", userdir))

		app.Stop()
	})

	u.list.SetBorder(true).SetTitle("Nodes (Ctrl+n)")
	u.list.SetCurrentOption(0)

}

func (u *MainUI) defineNodes(r []apiname) {
	tmpl, _ := template.New("view").Parse(configtemplate)
	for i, n := range r {
		var b bytes.Buffer
		name := n.Name.Last
		mac := fmt.Sprintf("%s/.lndev/user%d/data/chain/bitcoin/regtest/admin.macaroon", userdir, i+1)
		view := &cfgview{}
		view.N = i + 1
		view.Rpc = view.N + BASE_PORT
		view.Listen = view.N + BASE_PORT + 1000
		view.Rest = view.N + BASE_PORT + 2000
		view.Macaroon = mac
		view.Name = n.Name.Last
		view.User = userdir
		err := tmpl.Execute(&b, view)
		if err != nil {
			panic(err)
		}
		cmd := fmt.Sprintf("lncli --rpcserver=localhost:%d --macaroonpath=%s/.lndev/user%d/data/chain/bitcoin/regtest/admin.macaroon", BASE_PORT+i+1, userdir, i+1)
		u.aliases[n.Name.Last] = &alias{&name, &cmd, BASE_PORT + i + 1, mac}

		udir := fmt.Sprintf("%s/.lndev/user%d", userdir, i+1)
		ensureDir(udir)

		f, err := os.Create(fmt.Sprintf("%s/.lndev/user%d/lnd.conf", userdir, i+1))
		if err != nil {
			panic(err)
		}
		defer f.Close()
		_, err = f.Write(b.Bytes())

	}
	f, err := os.Create(fmt.Sprintf("%s/.lndev/bitcoin/bitcoin.conf", userdir))
	if err != nil {
		panic(err)
	}
	tmpl, _ = template.New("bitcoin").Parse(bitcoinconf)
	var b bytes.Buffer
	err = tmpl.Execute(&b, userdir)
	defer f.Close()
	_, err = f.Write(b.Bytes())
}
