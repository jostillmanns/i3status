package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/guelfey/go.dbus"
	"os/exec"
)

type Module struct {
	Name     string `json:"name"`
	Instance string `json:"instance"`
	Color    string `json:"color"`
	FullText string `json:"full_text"`
}

func Bluetooth(conn *dbus.Conn) []map[string]dbus.Variant {
	obj := conn.Object("org.bluez", "/")

	call := obj.Call("org.bluez.Manager.ListAdapters", 0)
	if call.Err != nil {
		panic(call.Err)
	}

	var adapter []dbus.ObjectPath
	err := call.Store(&adapter)
	if err != nil {
		panic(adapter)
	}

	var devices []dbus.ObjectPath
	for _, e := range adapter {
		obj := conn.Object("org.bluez", e)
		call := obj.Call("org.bluez.Adapter.ListDevices", 0)
		if call.Err != nil {
			panic(call.Err)
		}

		var dev []dbus.ObjectPath
		err = call.Store(&dev)
		if err != nil {
			panic(err)
		}

		cat := make([]dbus.ObjectPath, len(devices)+len(dev))
		copy(cat, devices)
		copy(cat[len(devices):], dev)

		devices = cat
	}
	props := make([]map[string]dbus.Variant, 0)

	for _, e := range devices {
		obj := conn.Object("org.bluez", e)
		call := obj.Call("org.bluez.Device.GetProperties", 0)
		if call.Err != nil {
			panic(call.Err)
		}

		dict := make(map[string]dbus.Variant)
		err = call.Store(&dict)
		if err != nil {
			panic(err)
		}

		props = append(props, dict)
	}

	return props
}

func Default(buffer chan []Module, confpath string) {
	cmd := exec.Command("/usr/bin/i3status", "-c", confpath)
	reader, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	cmd.Start()

	for {
		bufreader := bufio.NewReader(reader)
		line, _, err := bufreader.ReadLine()
		if err != nil {
			// fmt.Println(err)
			buffer <- []Module{Module{FullText: "error"}}
			continue
		}
		line = bytes.TrimLeft(line, ",")

		var modules []Module
		err = json.Unmarshal(line, &modules)

		if err != nil {
			buffer <- []Module{Module{FullText: "error"}}
			continue
		}
		buffer <- modules
	}
}

func main() {
	var (
		defaultChannel chan []Module
		defaultModules []Module
		btModules      []Module
		modules        []Module

		confpath string
	)
	flag.StringVar(&confpath, "c", "", "path to original i3status conf file")
	flag.Parse()
	// Print Version and json array starting point
	fmt.Println("{\"version\":1}\n[")

	defaultChannel = make(chan []Module)
	go Default(defaultChannel, confpath)

	//initiate dbus connection
	conn, err := dbus.SystemBus()
	if err != nil {
		panic(err)
	}

	first := true
	for {
		defaultModules = <-defaultChannel

		// correct colors
		for i, _ := range defaultModules {
			if len(defaultModules[i].Color) == 0 {
				defaultModules[i].Color = "#FFFFFF"
			}
		}

		btDevices := Bluetooth(conn)
		btModules = make([]Module, len(btDevices))
		for i, e := range btDevices {
			color := "FF0000"
			if e["Connected"].Value().(bool) {
				color = "00FF00"
			}
			btModules[i] = Module{
				Name:     e["Alias"].Value().(string),
				Instance: e["Address"].Value().(string),
				Color:    color,
				FullText: e["Alias"].Value().(string),
			}
		}

		modules = make([]Module, len(defaultModules)+len(btModules))
		copy(modules, btModules)
		copy(modules[len(btModules):], defaultModules)

		output, err := json.Marshal(modules)
		if err != nil {
			panic(err)
		}

		if first {
			fmt.Println(string(output))
			first = false
			continue
		}
		// add removed array delimiter
		fmt.Println("," + string(output))
	}
}
