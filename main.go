package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudfoundry/bytefmt"
	emitter "github.com/emitter-io/go"
	ui "github.com/gizak/termui"
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
}

var top = newTable()

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

	// calculate layout
	ui.Body.Align()
	ui.Render(ui.Body)
	ui.Loop() // block until StopLoop is called
}

// Occurs when a status is received
func onStatusReceived(client emitter.Emitter, msg emitter.Message) {
	defer func() {
		top.FgColors = top.FgColors[:0]
		top.BgColors = top.BgColors[:0]
		top.Analysis()
		top.SetSize()

		top.Border = true
		ui.Body.Align()
		ui.Render(ui.Body)
	}()

	stats := new(StatusInfo)
	if err := json.Unmarshal(msg.Payload(), stats); err == nil {
		row := []string{
			fmt.Sprintf("%02d:%03d", stats.Time.Second(), stats.Time.Nanosecond()/1000000),
			stats.Node,
			stats.Addr,
			fmt.Sprintf("%.2f%%", stats.CPU),
			fmt.Sprintf("%v", bytefmt.ByteSize(stats.MemoryPrivate)),
			fmt.Sprintf("%d", stats.Subscriptions),
		}

		for i, r := range top.Rows {
			if r[1] == stats.Node {
				top.Rows[i] = row
				return
			}
		}

		top.Rows = append(top.Rows, row)
	}
}

func newTable() *ui.Table {
	top := ui.NewTable()
	top.Rows = [][]string{[]string{"Time", "Node", "Addr", "CPU", "Mem", "Subs"}}
	top.FgColor = ui.ColorWhite
	top.BgColor = ui.ColorDefault
	top.TextAlign = ui.AlignCenter
	top.Border = false
	top.BorderLabel = "CLUSTER STATUS"

	top.Separator = false
	return top
}
