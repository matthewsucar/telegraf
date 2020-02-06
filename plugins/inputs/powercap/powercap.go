package powercap

import (
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
)

type Powercap struct {
	Log telegraf.Logger `toml:"-"`
	Sysfspath string `toml:sysfspath`
	DeviceList []Device `toml:"-"`
}

type Device struct {
	Driver string
	Name string
	Path string
	Device string
}

func (pc *Powercap) Description() string {
	return "Collect power usage statistics from the Linux powercap driver"
}

func (pc *Powercap) SampleConfig() string {
	return `
  sysfspath = "/sys/devices/virtual/powercap"
`
}

func (pc *Powercap) processDevice(driver string, path string) {
	if driver == "power" {
		return
	}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		pc.Log.Errorf("Error Opening Powercap device: %s reason: %s", driver, err)
		return
	}
	var d Device
	for _, f := range files {
		if f.IsDir() {
			if f.Name() != "power" {
				pc.processDevice(driver, filepath.Join(path, f.Name()))
			}
		}
		if f.Name() == "name" {
			namepath := filepath.Join(path, f.Name())
			name, err := ioutil.ReadFile(namepath)
			if err != nil {
				pc.Log.Errorf("Error reading device name: %s readon: %s", namepath, err)
			}
			pc.Log.Debugf("Found: %s", name)
			d.Name = strings.TrimSpace(string(name))
		}
		if f.Name() == "energy_uj" {
			d.Path = path
		}
	}
	if d.Name == "" {
		pc.Log.Debugf("No Name Found in: %s", path)
		return
	}
	d.Device = filepath.Base(path)
	d.Driver = driver
	pc.DeviceList = append(pc.DeviceList, d)
}

func (pc *Powercap) Init() error {
	drivers, err := ioutil.ReadDir(pc.Sysfspath)
	if err != nil {
		pc.Log.Errorf("Unable to open Powercap driver: %s, reason: %s", pc.Sysfspath,err)
		return err
	}
	for _,driver := range drivers {
		if driver.IsDir() {
			pc.processDevice(driver.Name(), filepath.Join(pc.Sysfspath,driver.Name()))
		}
	}

	return nil
}

func (pc *Powercap) Gather(acc telegraf.Accumulator) error {
	for _, d := range pc.DeviceList {
		energy, err := ioutil.ReadFile(filepath.Join(d.Path, "energy_uj"))
		if err != nil {
			pc.Log.Errorf("No energy provided for %s, skipping", d.Path)
			continue
		}
		uj, err := strconv.ParseUint(strings.TrimSpace(string(energy)), 10, 64)
		if err != nil {
			pc.Log.Errorf("Error parsing energy_uj, skipping %s reason: %s", d.Path, err)
			continue
		}
		acc.AddFields("powercap", map[string]interface{}{"energy_uj":uj},
			map[string]string{"device":d.Device, "driver":d.Driver, "name":d.Name})
	}
	return nil
}

func init() {
	inputs.Add("powercap", func() telegraf.Input { return &Powercap{} })
}