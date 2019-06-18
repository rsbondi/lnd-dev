package main

import (
	"bytes"
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
	"os"
	"os/exec"
	"strings"
	"text/template"
)

type MainUI struct {
	cli                     *tview.InputField
	list                    *tview.DropDown
	cliresult               *tview.TextView
	currentnode, workingdir string
	aliases                 map[string]*alias
	nodes                   map[string]*node
}

func NewMainUI() *MainUI {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	workingdir := dir
	dir = fmt.Sprintf("%s/profiles", dir)
	ensureDir(dir)

	ui := &MainUI{
		cliresult:  tview.NewTextView().SetDynamicColors(true),
		cli:        tview.NewInputField(),
		list:       tview.NewDropDown(),
		workingdir: workingdir,
		aliases:    make(map[string]*alias),
		nodes:      make(map[string]*node),
	}
	ui.cliresult.SetBorder(false).SetTitle("CLI Result (Ctrl+y)")

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
		SetFieldWidth(0).SetBorder(true).SetTitle("CLI (Ctrl+i) for CLI (Ctrl+y) for results")

	ui.cli.SetInputCapture(ui.cliInputCapture)

	return ui

}

func (u *MainUI) cliInputCapture(key *tcell.EventKey) *tcell.EventKey {
	if key.Key() == tcell.KeyEnter {
		text := u.cli.GetText()
		cmdfmt := fmt.Sprintf("[#ff0000]# %s[white]\n", text)
		fmt.Fprintf(u.cliresult, cmdfmt)
		if text == "" {
			fmt.Fprintf(u.cliresult, "Please provide a command to execute\n")
			return key
		}
		args, err := parseCommandLine(text)
		if err != nil {
			fmt.Fprintf(u.cliresult, "%s\n", err.Error())
		}

		cmd := u.aliases[u.currentnode].Command(args...)
		cmd.Stdin = strings.NewReader("some input")
		var out bytes.Buffer
		cmd.Stdout = &out
		err = cmd.Run()
		if err != nil {
			fmt.Fprintf(u.cliresult, "%s\n", err.Error())
		}

		fmt.Fprintf(u.cliresult, "%s\n", tview.Escape(out.String()))
		u.nodes[u.currentnode].Buff += cmdfmt
		u.nodes[u.currentnode].Buff += out.String()
		u.nodes[u.currentnode].Cmds = append(u.nodes[u.currentnode].Cmds, u.cli.GetText())
		*u.nodes[u.currentnode].CmdIndex = len(u.nodes[u.currentnode].Cmds)

		u.cli.SetText("")
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

	confcmd := fmt.Sprintf("bitcoin-cli -conf=%s/bitcoin.conf", u.workingdir)
	name := "Regtest"
	u.aliases[name] = &alias{&name, &confcmd}
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
		cmd := exec.Command("bitcoin-cli", fmt.Sprintf("-conf=%s/bitcoin.conf", u.workingdir), "stop")
		cmd.Run()

		for i := 1; i < len(u.aliases); i++ {
			host := fmt.Sprintf("--rpcserver=localhost:%d", BASE_PORT+i)
			macaroon := fmt.Sprintf("--macaroonpath=profiles/user%d/data/chain/bitcoin/regtest/admin.macaroon", i)
			cmd := exec.Command("lncli", host, macaroon, "stop")
			cmd.Run()
		}

		os.RemoveAll(fmt.Sprintf("%s/profiles", u.workingdir))

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
		view := &cfgview{}
		view.N = i + 1
		view.Name = n.Name.Last
		err := tmpl.Execute(&b, view)
		if err != nil {
			panic(err)
		}
		cmd := fmt.Sprintf("lncli --rpcserver=localhost:%d --macaroonpath=profiles/user%d/data/chain/bitcoin/regtest/admin.macaroon", BASE_PORT+i+1, i+1)
		u.aliases[n.Name.Last] = &alias{&name, &cmd}

		udir := fmt.Sprintf("profiles/user%d", i+1)
		ensureDir(udir)

		f, err := os.Create(fmt.Sprintf("profiles/user%d/lnd.conf", i+1))
		if err != nil {
			panic(err)
		}
		defer f.Close()
		_, err = f.Write(b.Bytes())
	}
}
