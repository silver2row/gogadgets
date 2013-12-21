package gogadgets

import (
	"log"
	"fmt"
	"time"
	"errors"
	"regexp"
	"strings"
	"strconv"
)

var (
	timeExp = regexp.MustCompile(`for (\d*\.?\d*) (seconds?|minutes?|hours?)`)
	stepExp = regexp.MustCompile(`for (.+) (>=|>|==|<=|<) (\d*\.?\d*)`)
)

type stepChecker func(msg *Message) bool
type comparitor func(value float64) bool

type Runner struct {
	Gadget
	method []string
	waitTime time.Duration
	stepChecker stepChecker
	step int
	out chan<- Message
}

func (m *Runner) Start(in <-chan Message, out chan<- Message) {
	m.waitTime = time.Duration(10000 * time.Hour)
	m.out = out
	shutdown := false
	for !shutdown {
		select {
		case msg := <-in:
			shutdown = m.readMessage(&msg)
		case <-time.After(m.waitTime):
			m.waitTime = time.Duration(10000 * time.Hour)
			m.runNextStep()
		}
	}
	m.out<- Message{}
}

func (m *Runner) readMessage(msg *Message) (shutdown bool) {
	if msg.Type == METHOD {
		m.method = msg.Method
		m.step = -1
		m.runNextStep()
		shutdown = false
	} else if len(m.method) != 0 && msg.Type == UPDATE {
		m.checkUpdate(msg)
		shutdown = false
	} else if msg.Type == COMMAND && msg.Body == "shutdown" {
		shutdown = true
	} else {
		shutdown = false
	}
	return shutdown
}

func (m *Runner) checkUpdate(msg *Message) {
	if m.stepChecker != nil && m.stepChecker(msg) {
		m.stepChecker = nil
		m.runNextStep()
	}
}

func (m *Runner) runNextStep() {
	m.step += 1
	if len(m.method) <= m.step {
		m.method = []string{}
		m.step = -1
		return
	}
	cmd := m.method[m.step]
	if strings.Index(cmd, "wait") == 0 {
		m.readWaitCommand(cmd)
	} else {
		m.sendCommand(cmd)
		m.runNextStep()
	}
}

func (m *Runner) sendCommand(cmd string) {
	msg := Message{
		Type: COMMAND,
		Body: cmd,
	}
	m.out<- msg
}

func (m *Runner) readWaitCommand(cmd string) {
	result := timeExp.FindStringSubmatch(cmd)
	if len(result) == 3 {
		m.setWaitTime(result)
	} else {
		m.setStepChecker(cmd)
	}
}

func (m *Runner) setStepChecker(cmd string) {
	uid, operator, value, err := m.parseWaitCommand(cmd)
	if err == nil {
		compare, err := m.getCompare(operator, value)
		if err == nil {
			m.stepChecker = func(msg *Message) bool {
				val, ok := msg.Value.Value.(float64)
				return ok &&
					msg.Sender == uid &&
					compare(val)
			}
		} else {
			log.Println(err)
		}
	}
}

func (m *Runner) getCompare(operator string, value float64) (cmp comparitor, err error) {
	if operator == "<=" {
		cmp = func(x float64) bool {return x <= value}
	} else if operator == "<" {
		cmp = func(x float64) bool {return x < value}
	} else if operator == "==" {
		cmp = func(x float64) bool {return x == value}
	} else if operator == ">=" {
		cmp = func(x float64) bool {return x >= value}
	} else if operator == ">" {
		cmp = func(x float64) bool {return x > value}
	} else {
		err = errors.New(fmt.Sprintf("invalid operator: %s", operator))
	}
	return cmp, err
}

func (m *Runner) parseWaitCommand(cmd string) (uid string, operator string, value float64, err error) {
	result := stepExp.FindStringSubmatch(cmd)
	if len(result) == 4 {
		uid = result[1]
		operator = result[2]
		value, err = strconv.ParseFloat(result[3], 64)
	}
	return uid, operator, value, err
}

func (m *Runner) setWaitTime(cmd []string) {
	units := cmd[2]
	t, err := strconv.ParseFloat(cmd[1], 64)
	if err != nil {
		log.Println("could not parse command", cmd)
	} else {
		if units == "minutes" || units == "minute" {
			t *= 60.0
		} else if units == "hours" || units == "hour" {
			t *= 3600.0
		}
		m.waitTime = time.Duration(t * float64(time.Second))
	}
}