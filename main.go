package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	ui "github.com/LINBIT/termui"
	"github.com/cloudfoundry/bytefmt"
	"github.com/emitter-io/emitter/utils"
	emitter "github.com/emitter-io/go"
)

// StatusInfo represents the status payload.
type StatusInfo struct {
	Node          string    `json:"node"`
	Addr          string    `json:"addr"`
	Subscriptions int       `json:"subs"`
	CPU           float64   `json:"cpu"`
	MemoryPrivate uint64    `json:"priv"`
	MemoryVirtual uint64    `json:"virt"`
	Time          time.Time `json:"time"`
	Uptime        float64   `json:"uptime"`
	NumPeers      int       `json:"peers"`
}

var top = newTable()
var data = new(sync.Map)

func main() {
	err := ui.Init()
	if err != nil {
		panic(err)
	}
	defer ui.Close()

	// Create the options with default values
	o := emitter.NewClientOptions()
	o.AddBroker("tcp://127.0.0.1:8080")
	o.SetOnMessageHandler(onStatusReceived)

	// Create a new emitter client and connect to the broker
	c := emitter.NewClient(o)
	sToken := c.Connect()
	if sToken.Wait() && sToken.Error() != nil {
		panic("Error on Client.Connect(): " + sToken.Error().Error())
	}

	// Subscribe to the cluster channel
	c.Subscribe("1RszYitFOWDlzKhhqaxDG8--vw4RbCTt", "cluster/")

	// press q to quit
	ui.Handle("/sys/kbd/q", func(ui.Event) {
		ui.StopLoop()
	})

	ui.Handle("/sys/kbd/C-c", func(ui.Event) {
		ui.StopLoop()
	})

	// build
	top = newTable()
	ui.Body.AddRows(
		ui.NewRow(ui.NewCol(12, 0, top)),
	)

	closing := make(chan bool)
	utils.Repeat(render, 100*time.Millisecond, closing)

	// calculate layout
	ui.Body.Align()
	ui.Render(ui.Body)
	ui.Loop() // block until StopLoop is called
}

// Occurs when a status is received
func onStatusReceived(client emitter.Emitter, msg emitter.Message) {
	stats := new(StatusInfo)
	if err := json.Unmarshal(msg.Payload(), stats); err == nil {
		data.Store(stats.Node, stats)
	}
}

// render redraws the table
func render() {
	r := [][]string{}
	data.Range(func(k, v interface{}) bool {
		stats := v.(*StatusInfo)
		r = append(r, []string{
			fmt.Sprintf("%02d:%03d", stats.Time.Second(), stats.Time.Nanosecond()/1000000),
			stats.Node,
			stats.Addr,
			fmt.Sprintf("%d", stats.NumPeers),
			fmt.Sprintf("%.2f%%", stats.CPU),
			fmt.Sprintf("%v", bytefmt.ByteSize(stats.MemoryPrivate)),
			fmt.Sprintf("%d", stats.Subscriptions),
		})
		return true
	})

	sort.Slice(r, func(i, j int) bool {
		return strings.Compare(r[i][1], r[j][1]) < 0
	})

	rows := [][]string{[]string{"Time", "Node", "Addr", "Peers", "CPU", "Mem", "Subs"}}
	rows = append(rows, r...)

	top.SetRows(rows)
	top.Analysis()
	top.SetSize()
	ui.Body.Align()
	ui.Render(ui.Body)
}

func newTable() *ui.Table {
	top := ui.NewTable()
	top.Rows = [][]string{[]string{"Loading..."}}
	top.FgColor = ui.ColorWhite
	top.BgColor = ui.ColorDefault
	top.TextAlign = ui.AlignCenter
	top.Border = true
	top.BorderLabel = "CLUSTER STATUS"
	top.Separator = false
	return top
}
