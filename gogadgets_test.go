package gogadgets_test

import (
	"time"

	"github.com/cswank/gogadgets"
)

type FakeOutput struct {
	on bool
}

func (f *FakeOutput) Config() gogadgets.ConfigHelper {
	return gogadgets.ConfigHelper{}
}

func (f *FakeOutput) Update(msg *gogadgets.Message) {

}

func (f *FakeOutput) On(val *gogadgets.Value) error {
	f.on = true
	return nil
}

func (f *FakeOutput) Off() error {
	f.on = false
	return nil
}

func (f *FakeOutput) Status() interface{} {
	return f.on
}

type FakePoller struct {
	trigger chan bool
	val     bool
}

func (f *FakePoller) Status() interface{} {
	return f.val
}

func (f *FakePoller) Wait() (bool, error) {
	if f.trigger == nil {
		time.Sleep(100 * time.Millisecond)
		f.val = !f.val
	} else {
		f.val = <-f.trigger
	}
	return f.val, nil
}
