package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"text/template"
)

var flex *tview.Flex
var form *tview.Form
var app *tview.Application
var cli *tview.InputField
var list *tview.DropDown
var cliresult *tview.TextView
var status *tview.TextView
var currentnode, nNodes, nChannels, workingdir string
var aliases map[string]*alias
var nodes map[string]*node
var commands []*exec.Cmd

func main() {
	cliresult = tview.NewTextView().SetDynamicColors(true)
	status = tview.NewTextView()
	cli = tview.NewInputField()
	list = tview.NewDropDown()

	app = tview.NewApplication()
	currentnode = ""

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
	col.AddItem(list, 40, 1, false)
	col.AddItem(cli, 0, 1, true)
	flex.AddItem(col, 3, 1, true)
	flex.AddItem(cliresult, 0, 5, false)
	flex.RemoveItem(form)
	flex.RemoveItem(status)
}

func defineNodes(r []apiname) map[string]*alias {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	workingdir = dir
	dir = fmt.Sprintf("%s/profiles", dir)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}
	}

	tmpl, _ := template.New("view").Parse(configtemplate)
	aliases = make(map[string]*alias)

	for i, n := range r {
		var b bytes.Buffer
		name := n.Name.Last
		view := &cfgview{}
		view.N = i + 1
		view.Name = n.Name.Last
		err = tmpl.Execute(&b, view)
		cmd := fmt.Sprintf("lncli --rpcserver=localhost:1000%d --macaroonpath=profiles/user%d/data/chain/bitcoin/regtest/admin.macaroon", i+1, i+1)
		aliases[n.Name.Last] = &alias{&name, &cmd}

		f, err := os.Create(fmt.Sprintf("profiles/user%d", i+1))
		if err != nil {
			panic(err)
		}
		defer f.Close()
		_, err = f.Write(b.Bytes())
	}
	return aliases

}

func cliInputCapture(key *tcell.EventKey) *tcell.EventKey {
	if key.Key() == tcell.KeyEnter {
		text := cli.GetText()
		cmdfmt := fmt.Sprintf("[#ff0000]# %s[white]\n", text)
		fmt.Fprintf(cliresult, cmdfmt)
		if text == "" {
			fmt.Fprintf(cliresult, "Please provide a command to execute\n")
			return key
		}
		args, err := parseCommandLine(text)
		if err != nil {
			fmt.Fprintf(cliresult, "%s\n", err.Error())
		}
		clicmd := strings.Split(*aliases[currentnode].Path, " ")
		cliarg := []string{clicmd[1]}
		cliargs := append(cliarg, args...)
		cmd := exec.Command(clicmd[0], cliargs...)
		cmd.Stdin = strings.NewReader("some input")
		var out bytes.Buffer
		cmd.Stdout = &out
		err = cmd.Run()
		if err != nil {
			fmt.Fprintf(cliresult, "%s\n", err.Error())
		}

		fmt.Fprintf(cliresult, "%s\n", tview.Escape(out.String()))
		nodes[currentnode].Buff += cmdfmt
		nodes[currentnode].Buff += out.String()
		nodes[currentnode].Cmds = append(nodes[currentnode].Cmds, cli.GetText())
		*nodes[currentnode].CmdIndex = len(nodes[currentnode].Cmds)

		cli.SetText("")
	} else if key.Key() == tcell.KeyUp {
		index := nodes[currentnode].CmdIndex
		if *index > 0 {
			*index = *index - 1
		}
		if *index >= 0 && *index < len(nodes[currentnode].Cmds) {
			cli.SetText(nodes[currentnode].Cmds[*index])
		}
	} else if key.Key() == tcell.KeyDown {
		index := nodes[currentnode].CmdIndex
		if *index == len(nodes[currentnode].Cmds)-1 {
			cli.SetText("")
			*index = *index + 1
			return key
		}
		if *index < len(nodes[currentnode].Cmds)-1 {
			*index = *index + 1
		}
		if *index >= 0 && *index < len(nodes[currentnode].Cmds) {
			cli.SetText(nodes[currentnode].Cmds[*index])
		}
	} else if key.Key() == tcell.KeyCtrlV {
		clip, err := clipboard.ReadAll()
		if err != nil {
			fmt.Fprintf(cliresult, "%s\n", err.Error())
		} else {
			full := strings.Replace(clip, "\n", "", -1)
			cli.SetText(fmt.Sprintf("%s%s", cli.GetText(), full)) // TODO: this only paste to end, fix for insert
		}
	}
	return key
}

func populateList() {
	aliasKeys := sortAliasKeys(aliases)
	for _, a := range aliasKeys {
		s := -1
		anode := &node{"", []string{}, &s}
		nodes[*aliases[a].Name] = anode

		name := *aliases[a].Name
		list.AddOption(*aliases[a].Name, func() {
			cli.SetText("")
			currentnode = name
			cliresult.SetText(nodes[name].Buff)
			app.SetFocus(cli)
		})
	}

	confcmd := fmt.Sprint("bitcoin-cli --conf=./bitcoin.conf")
	name := "Regtest"
	aliases[name] = &alias{&name, &confcmd}
	s := -1
	anode := &node{"", []string{}, &s}
	nodes[name] = anode

	list.AddOption(name, func() {
		cli.SetText("")
		currentnode = name
		cliresult.SetText(nodes[name].Buff)
		app.SetFocus(cli)
	})
	list.AddOption("Quit", func() {
		// kill bitcoind
		cmd := exec.Command("bitcoin-cli", fmt.Sprintf("-conf=%s/bitcoin.conf", workingdir), "stop")
		cmd.Run()

		// kill all lnd instances
		for _, c := range commands {
			fmt.Fprintf(cliresult, "Killing %d: %q\n", c.Process.Pid, c.Args)
			if err := syscall.Kill(c.Process.Pid, syscall.SIGKILL); err != nil {
				panic(fmt.Sprintf("failed to kill process: %s", err.Error()))
			}

		}
		app.Stop()
	})

	list.SetBorder(true).SetTitle("Nodes (Ctrl+n)")
	list.SetCurrentOption(0)

}

func setUI() {
	names := randomNames()

	nodes = make(map[string]*node)

	cliresult.SetBorder(false).SetTitle("CLI Result (Ctrl+y)")

	cliresult.SetInputCapture(func(key *tcell.EventKey) *tcell.EventKey {
		if key.Key() == tcell.KeyCtrlL {
			cliresult.SetText("")
			nodes[currentnode].Buff = ""
		}
		return key
	})

	aliases = defineNodes(names.Results)

	populateList()

	cli.
		SetPlaceholder("Enter cli command - use Ctrl+v to paste (no shift)").
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetFieldWidth(0).SetBorder(true).SetTitle("CLI (Ctrl+i) for CLI (Ctrl+y) for results")

	cli.SetInputCapture(cliInputCapture)

	app.SetInputCapture(func(key *tcell.EventKey) *tcell.EventKey {
		if key.Key() == tcell.KeyCtrlN {
			app.SetFocus(list)
			list.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, '0', tcell.ModNone), func(tview.Primitive) { app.SetFocus(list) })
		} else if key.Key() == tcell.KeyCtrlI {
			cli.SetText("")
			app.SetFocus(cli)
		} else if key.Key() == tcell.KeyCtrlY {
			app.SetFocus(cliresult)
		}
		return key
	})

	launchNodes()
}

func launchNodes() {
	fmt.Fprintln(status, "launching bitcoin node")

	cmd := exec.Command("bitcoind", fmt.Sprintf("-conf=%s/bitcoin.conf", workingdir))

	err := cmd.Start()

	if err != nil {
		fmt.Fprintf(status, "%s\n", err.Error())
	}

	u := 1
	for _, v := range aliases {
		// TODO: actually launch
		fmt.Fprintf(status, "launching node for %s\n with lnd --configfile=profiles/user%d command=%s\n", *v.Name, u, *v.Path)
		//	commands = append(commands, cmd)
		u++
	}
	swapForm()
}
