package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
	"net/http"
	"os"
	"os/exec"
	"time"
)

var flex *tview.Flex
var form *tview.Form
var app *tview.Application
var status *tview.TextView
var ui *MainUI
var nNodes, nChannels string

func main() {
	ui = NewMainUI()
	status = tview.NewTextView()

	app = tview.NewApplication()

	form = tview.NewForm().
		AddInputField("Number of Nodes", "", 5, tview.InputFieldInteger, func(t string) {
			nNodes = t
		}).
		AddInputField("Incoming Connections per Node", "", 5, tview.InputFieldInteger, func(t string) {
			nChannels = t
		}).
		AddButton("Ok", setUI).
		AddButton("Cancel", func() {
			app.Stop()
		})
	form.SetBorder(true).SetTitle("Enter node data").SetTitleAlign(tview.AlignLeft)

	flex = tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(form, 0, 5, true)
	flex.AddItem(status, 0, 5, false)

	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
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

func swapForm() {
	col := tview.NewFlex().SetDirection(tview.FlexColumn)
	col.AddItem(ui.list, 40, 1, false)
	col.AddItem(ui.cli, 0, 1, true)
	flex.AddItem(col, 3, 1, true)
	flex.AddItem(ui.cliresult, 0, 5, false)
	flex.RemoveItem(form)
	flex.RemoveItem(status)
}

func ensureDir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}
	}
}

func setUI() {
	names := randomNames()

	ui.populateList(names.Results)

	app.SetInputCapture(func(key *tcell.EventKey) *tcell.EventKey {
		if key.Key() == tcell.KeyCtrlN {
			app.SetFocus(ui.list)
			ui.list.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, '0', tcell.ModNone), func(tview.Primitive) { app.SetFocus(ui.list) })
		} else if key.Key() == tcell.KeyCtrlI {
			ui.cli.SetText("")
			app.SetFocus(ui.cli)
		} else if key.Key() == tcell.KeyCtrlY {
			app.SetFocus(ui.cliresult)
		}
		return key
	})

	launchNodes()
}

func launchNodes() {
	fmt.Fprintln(status, "launching bitcoin node")

	cmd := exec.Command("bitcoind", fmt.Sprintf("-conf=%s/bitcoin.conf", ui.workingdir))

	err := cmd.Start()

	if err != nil {
		fmt.Fprintf(status, "%s\n", err.Error())
	}

	time.Sleep(2 * time.Second)
	u := 1
	for _, v := range ui.aliases {
		if *v.Name == "Regtest" {
			continue
		}
		fmt.Fprintf(status, "launching node for %s\n with lnd --configfile=%s/profiles/user%d/lnd.conf command=%s\n", *v.Name, ui.workingdir, u, *v.Path)
		cmd := exec.Command("lnd", fmt.Sprintf("--configfile=%s/profiles/user%d/lnd.conf", ui.workingdir, u))

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
	swapForm()
}
