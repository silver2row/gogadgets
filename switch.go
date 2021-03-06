package gogadgets

import (
	"log"
	"time"
)

//Switch is an input device that waits for a GPIO pin
//to change value (1 to 0 or 0 to 1).  When that change
//happens it sends an update to the rest of the system.
type Switch struct {
	GPIO       Poller
	Value      interface{}
	TrueValue  interface{}
	FalseValue interface{}
	Units      string
	out        chan<- Value
}

func NewSwitch(pin *Pin) (InputDevice, error) {
	pin.Direction = "in"
	var err error
	var s *Switch
	gpio, err := NewGPIO(pin)
	if err != nil {
		return nil, err
	}

	s = &Switch{
		GPIO:      gpio,
		TrueValue: pin.Value,
		Units:     pin.Units,
	}
	switch s.TrueValue.(type) {
	case bool:
		s.Value = false
		s.FalseValue = false
	default:
		s.Value = float64(0.0)
		s.FalseValue = float64(0.0)
	}
	return s, nil
}

func (s *Switch) Config() ConfigHelper {
	return ConfigHelper{
		PinType: "pwm",
		Pins:    Pins["gpio"],
		Fields: map[string][]string{
			"edge": []string{"rising", "falling", "both"},
		},
	}
}

//The GPIO does the real waiting here.  This wraps it and adds
//a delay so that the inevitable bounce in the signal from the
//physical device is ignored.
func (s *Switch) wait(out chan<- interface{}, err chan<- error) {
	for {
		val, e := s.GPIO.Wait()
		if e != nil {
			err <- e
			return
		}
		switch v := s.TrueValue.(type) {
		case bool:
			out <- val
		default:
			if val {
				out <- v
			} else {
				out <- 0.0
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *Switch) readValue() {
	v := s.GPIO.Status()
	if v["gpio"] {
		s.Value = s.TrueValue
	} else {
		s.Value = s.FalseValue
	}
}

func (s *Switch) SendValue() {
	s.out <- Value{
		Value: s.Value,
		Units: s.Units,
	}
}

func (s *Switch) GetValue() *Value {
	return &Value{
		Value: s.Value,
		Units: s.Units,
	}
}

func (s *Switch) Start(in <-chan Message, out chan<- Value) {
	s.out = out
	value := make(chan interface{})
	err := make(chan error)
	s.readValue()
	s.SendValue()
	keepGoing := true
	go s.wait(value, err)
	for keepGoing {
		select {
		case <-in:
			//do nothing
		case val := <-value:
			s.Value = val
			s.SendValue()
		case e := <-err:
			log.Println(e)
		}
	}
}
