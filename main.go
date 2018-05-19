package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/bytefmt"
	"github.com/emitter-io/emitter/async"
	"github.com/emitter-io/emitter/monitor"
	"github.com/emitter-io/emitter/network/address"
	emitter "github.com/emitter-io/go"
	"github.com/gdamore/tcell"
	"github.com/jessevdk/go-flags"
	"github.com/rivo/tview"
)

var opts struct {
	Broker string `short:"b" long:"broker" description:"The address of a broker in a IP:Port format" default:"127.0.0.1:8080"`
	Key    string `short:"k" long:"key" description:"The key for the cluster channel" required:"true"`
}

var app = tview.NewApplication()
var data = new(sync.Map)
var table = tview.NewTable().
	SetBorders(true).
	SetFixed(1, 1)

func main() {
	if _, err := flags.ParseArgs(&opts, os.Args); err != nil {
		os.Exit(1)
	}

	// Create the options with default values
	o := emitter.NewClientOptions()
	o.AddBroker("tcp://" + opts.Broker)
	o.SetOnMessageHandler(onStatusReceived)

	// Create a new emitter client and connect to the broker
	c := emitter.NewClient(o)
	sToken := c.Connect()
	if sToken.Wait() && sToken.Error() != nil {
		panic("Error on Client.Connect(): " + sToken.Error().Error())
	}

	// Subscribe to the cluster channel
	c.Subscribe(opts.Key, "stats/")

	async.Repeat(context.Background(), 100*time.Millisecond, render)

	flex := tview.NewFlex().AddItem(table, 0, 1, true)
	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}

// Occurs when a status is received
func onStatusReceived(client emitter.Emitter, msg emitter.Message) {
	if metrics, err := monitor.Restore(msg.Payload()); err == nil {
		stats := make(map[string]*monitor.Snapshot)
		for _, m := range metrics {
			copy := m // Don't capture the iterator
			stats[m.Name()] = &copy
		}

		node := address.Fingerprint(uint64(stats["node.id"].Max())).String()
		data.Store(node, stats)
	}
}

// render redraws the table
func render() {
	rows := [][]string{}
	data.Range(func(k, v interface{}) bool {
		stat := func(k string) *monitor.Snapshot {
			if s, ok := v.(map[string]*monitor.Snapshot)[k]; ok {
				return s
			}
			return new(monitor.Snapshot)
		}

		rows = append(rows, []string{
			fmt.Sprintf("%s", k),
			fmt.Sprintf("%d", stat("node.peers").Max()),
			fmt.Sprintf("%d", stat("node.conns").Max()),
			fmt.Sprintf("x%d", stat("go.procs").Max()),
			fmt.Sprintf("%d", stat("go.count").Max()),
			fmt.Sprintf("%v/%v",
				bytefmt.ByteSize(uint64(stat("heap.inuse").Max())),
				bytefmt.ByteSize(uint64(stat("heap.sys").Max()))),
			fmt.Sprintf("%v %.1f%%",
				bytefmt.ByteSize(uint64(stat("gc.sys").Max())),
				stat("gc.cpu").Mean()/10),
			fmt.Sprintf("%.0fμs", stat("send.pub").Quantile(99)[0]),
			fmt.Sprintf("%.0fμs", stat("rcv.pub").Quantile(99)[0]),
		})
		return true
	})

	sort.Slice(rows, func(i, j int) bool {
		return strings.Compare(rows[i][0], rows[j][0]) < 0
	})

	headers := []string{"Addr", "Peers", "Conns", "Core", "Go", "Heap", "GC", "<- p99", "-> p99"}
	for j, h := range headers {
		table.SetCell(0, j, tview.NewTableCell(h).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignCenter))
	}

	for i, items := range rows {
		for j, c := range items {
			table.SetCell(i+1, j, tview.NewTableCell(c).
				SetTextColor(tcell.ColorLightGrey).
				SetAlign(tview.AlignCenter))
		}
	}
	app.Draw()
}
