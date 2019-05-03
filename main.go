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
	emitter "github.com/emitter-io/go"
	"github.com/emitter-io/stats"
	"github.com/gdamore/tcell"
	"github.com/jessevdk/go-flags"
	"github.com/kelindar/etop/internal/async"
	"github.com/rivo/tview"
)

var opts struct {
	Broker string `short:"b" long:"broker" description:"The address of a broker in a protocol://IP:Port format" default:"tcp://127.0.0.1:8080"`
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
	o.AddBroker(opts.Broker)
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
	if metrics, err := stats.Restore(msg.Payload()); err == nil {
		snapshots := make(map[string]*stats.Snapshot)
		for _, m := range metrics {
			copy := m // Don't capture the iterator
			snapshots[m.Name()] = &copy
		}
		data.Store(snapshots["node.id"].Tag(), snapshots)
	}
}

// render redraws the table
func render() {
	rows := [][]string{}
	data.Range(func(k, v interface{}) bool {
		stat := func(k string) *stats.Snapshot {
			if s, ok := v.(map[string]*stats.Snapshot)[k]; ok {
				return s
			}
			return new(stats.Snapshot)
		}

		rows = append(rows, []string{
			fmt.Sprintf("%s", k),
			fmt.Sprintf("%s", stat("node.addr").Tag()),
			fmt.Sprintf("%v", (time.Duration(stat("proc.uptime").Max()) * time.Second).String()),
			fmt.Sprintf("%d", stat("node.peers").Max()),
			fmt.Sprintf("%d", stat("node.conns").Max()),
			fmt.Sprintf("%d", stat("node.subs").Max()),
			fmt.Sprintf("x%d", stat("go.procs").Max()),
			fmt.Sprintf("%d", stat("go.count").Max()),
			fmt.Sprintf("%v", size(stat("proc.priv").Max())),
			fmt.Sprintf("%v/%v",
				size(stat("heap.inuse").Max()),
				size(stat("heap.sys").Max())),
			fmt.Sprintf("%.1f%%", stat("gc.cpu").Mean()/100),
			fmt.Sprintf("%d ±%.0fμs",
				stat("send.pub").Count(),
				stat("send.pub").Quantile(99)[0]),
			fmt.Sprintf("%d ±%.0fμs",
				stat("rcv.pub").Count(),
				stat("rcv.pub").Quantile(99)[0]),
		})
		return true
	})

	sort.Slice(rows, func(i, j int) bool {
		return strings.Compare(rows[i][0], rows[j][0]) < 0
	})

	headers := []string{"ID", "Addr", "Uptime", "Peer", "Conn", "Subs", "Core", "Task", "Mem", "Heap", "GC", "Send", "Recv"}
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

// Size returns the size in bytes
func size(size int) string {
	return bytefmt.ByteSize(uint64(size))
}
