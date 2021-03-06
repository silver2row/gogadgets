// +build !windows

package gogadgets

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"syscall"

	"github.com/cswank/gogadgets/utils"
)

var (
	GPIO_DEV_PATH = "/sys/class/gpio"
	GPIO_DEV_MODE = os.ModeDevice
)

//GPIO interacts with the linux sysfs interface for GPIO
//to turn pins on and off.  The pins that are listed in
//gogadgets.Pins have been found to be availabe by default
//but by using the device tree overlay you can make more
//pins available.
//GPIO also has a Wait method and can poll a pin and wait
//for a change of direction.
type GPIO struct {
	units         string
	export        string
	exportPath    string
	directionPath string
	valuePath     string
	edgePath      string
	activeLowPath string
	direction     string
	edge          string
	activeLow     string
	fd            int
	fdSet         *syscall.FdSet
	buf           []byte
}

func GPIOFactory(pin *Pin) (OutputDevice, error) {
	g, err := NewGPIO(pin)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func NewGPIO(pin *Pin) (*GPIO, error) {
	var export string
	var ok bool
	if pin.Platform == "rpi" {
		export, ok = PiPins[pin.Pin]
		if !ok {
			return nil, fmt.Errorf("no such pin: %s", pin.Pin)
		}
	} else {
		var portMap map[string]string
		portMap, ok = Pins["gpio"][pin.Port]
		if !ok {
			return nil, fmt.Errorf("no such port: %s", pin.Port)
		}
		export, ok = portMap[pin.Pin]
		if !ok {
			return nil, fmt.Errorf("no such pin: %s", pin.Pin)
		}
	}
	if pin.Direction == "" {
		pin.Direction = "out"
	}
	g := &GPIO{
		export:        export,
		exportPath:    path.Join(GPIO_DEV_PATH, "export"),
		directionPath: path.Join(GPIO_DEV_PATH, fmt.Sprintf("gpio%s", export), "direction"),
		edgePath:      path.Join(GPIO_DEV_PATH, fmt.Sprintf("gpio%s", export), "edge"),
		valuePath:     path.Join(GPIO_DEV_PATH, fmt.Sprintf("gpio%s", export), "value"),
		activeLowPath: path.Join(GPIO_DEV_PATH, fmt.Sprintf("gpio%s", export), "active_low"),
		direction:     pin.Direction,
		activeLow:     pin.ActiveLow,
		edge:          pin.Edge,
	}
	err := g.Init()
	return g, err
}

func (g *GPIO) Commands(location, name string) *Commands {
	return nil
}

func (g *GPIO) Config() ConfigHelper {
	return ConfigHelper{
		PinType: "gpio",
		Pins:    Pins["gpio"],
	}
}

func (g *GPIO) Init() error {
	if !utils.FileExists(g.directionPath) {
		if err := g.writeValue(g.exportPath, g.export); err != nil {
			return err
		}
	}
	if g.activeLow == "1" {
		if err := g.writeValue(g.activeLowPath, g.activeLow); err != nil {
			return err
		}
	}
	if err := g.writeValue(g.directionPath, g.direction); err != nil {
		return err
	}
	if g.direction == "out" {
		if err := g.writeValue(g.valuePath, "0"); err != nil {
			return err
		}
	} else if g.edge != "" {
		if err := g.writeValue(g.edgePath, g.edge); err != nil {
			return err
		}
	}
	return nil
}

func (g *GPIO) Update(msg *Message) bool {
	return false
}

func (g *GPIO) On(val *Value) error {
	return g.writeValue(g.valuePath, "1")
}

func (g *GPIO) Status() map[string]bool {
	data, err := ioutil.ReadFile(g.valuePath)
	return map[string]bool{"gpio": err == nil && strings.Replace(string(data), "\n", "", -1) == "1"}
}

func (g *GPIO) Off() error {
	return g.writeValue(g.valuePath, "0")
}

func (g *GPIO) writeValue(path, value string) error {
	return ioutil.WriteFile(path, []byte(value), GPIO_DEV_MODE)
}

func (g *GPIO) Wait() (bool, error) {
	if g.fd == 0 {
		fd, err := syscall.Open(g.valuePath, syscall.O_RDONLY, 0666)
		if err != nil {
			return false, err
		}
		g.fd = fd
		g.fdSet = new(syscall.FdSet)
		FD_SET(g.fd, g.fdSet)
		g.buf = make([]byte, 64)
		syscall.Read(g.fd, g.buf)
	}
	syscall.Select(g.fd+1, nil, nil, g.fdSet, nil)
	syscall.Seek(g.fd, 0, 0)
	_, err := syscall.Read(g.fd, g.buf)
	if err != nil {
		return false, err
	}
	return string(g.buf[:2]) == "1\n", nil
}

func FD_SET(fd int, p *syscall.FdSet) {
	p.Bits[fd/32] |= 1 << (uint(fd) % 32)
}
