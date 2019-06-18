package main

import (
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
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

func swapForm() {
	col := tview.NewFlex().SetDirection(tview.FlexColumn)
	col.AddItem(ui.list, 40, 1, false)
	col.AddItem(ui.cli, 0, 1, true)
	flex.AddItem(col, 3, 1, true)
	flex.AddItem(ui.cliresult, 0, 5, false)
	flex.RemoveItem(form)
	flex.RemoveItem(status)
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

	launcher := NewLauncher(ui.workingdir, ui.aliases)
	launcher.launchNodes()
	swapForm()
}
