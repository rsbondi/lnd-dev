## lnd-dev - WIP

Generate a random development environment for lnd with established funding of channels

## Motivation
Inspired by [lnet](https://github.com/cdecker/lnet), this is a development tool that creates random nodes, connects them, funds channels, and provides a UI for managing.
You can begin development once launched without a complex procedure to get you started

## Features
* terminal based UI
* input amount of nodes and connections per node
* easily switch between nodes in UI
* UI includes bitcoin node
* all nodes properly shutdown on exit
* generate random invoices and payments between nodes

## Requirements
* bitcoind
* lnd

## Usage
1) Enter number of nodes
1) Enter maximum outbound channels
1) Enter number of random payments to generate
    * these will be running in the background
    * set to zero or leave blank if no activity desired

## UI Anomalies
* UI component copies wrapped lines with `\n` so standard `Ctrl-Shift-V` does not work with wrapped lines.  Use `Ctrl-V` instead
* Does not do interactive prompts so for example use `-f` or `--force` options with `payinvoice` or `sendpayment`, may be others?

## TODO:
* change `fmt.Sprintf`s to `path.Join` for windows

[See video](https://www.youtube.com/watch?v=47NPohE9WGU&feature=youtu.be)