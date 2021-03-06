package gogadgets

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	cronExp, _ = regexp.Compile("^[*0-9,-]+$")
)

type Afterer func(d time.Duration) <-chan time.Time

func NewCron(config *GadgetConfig, options ...func(*Cron) error) (*Cron, error) {

	c := &Cron{}

	for _, opt := range options {
		if err := opt(c); err != nil {
			return c, err
		}
	}

	if c.after == nil {
		c.after = time.After
	}

	if c.sleep == time.Duration(0) {
		c.sleep = time.Second
	}

	var err error
	if c.jobs == nil {
		v := config.Args["jobs"].([]interface{})
		jobs := make([]string, len(v))
		for i, r := range v {
			jobs[i] = r.(string)
		}
		c.jobs, err = c.parseJobs(jobs)
		if err != nil {
			return c, err
		}
	}

	return c, nil
}

func CronAfter(a Afterer) func(*Cron) error {
	return func(c *Cron) error {
		c.after = a
		return nil
	}
}

func CronSleep(d time.Duration) func(*Cron) error {
	return func(c *Cron) error {
		c.sleep = d
		return nil
	}
}

func CronJobs(j []string) func(*Cron) error {
	return func(c *Cron) error {
		var err error
		c.jobs, err = c.parseJobs(j)
		return err
	}
}

type Cron struct {
	after  Afterer
	sleep  time.Duration
	status bool
	jobs   map[string][]string
	out    chan<- Message
	ts     *time.Time
}

func (c *Cron) GetUID() string {
	return "cron"
}

func (c *Cron) GetDirection() string {
	return "na"
}

func (c *Cron) Start(in <-chan Message, out chan<- Message) {
	c.out = out
	for {
		select {
		case t := <-c.after(c.getSleep()):
			ts := time.Now()
			c.ts = &ts
			if t.Second() == 0 {
				c.checkJobs(t)
			}
		case <-in:
		}
	}
}

func (c *Cron) getSleep() time.Duration {
	if c.ts == nil {
		return c.sleep
	}
	diff := time.Now().Sub(*c.ts)
	return c.sleep - diff
}

func (c *Cron) parseJobs(jobs []string) (map[string][]string, error) {
	m := map[string][]string{}
	for _, row := range jobs {
		if err := c.parseJob(row, m); err != nil {
			return m, err
		}
	}
	return m, nil
}

func (c *Cron) parseJob(row string, m map[string][]string) error {
	if strings.Index(row, "#") == 0 {
		return nil
	}

	parts := strings.Fields(row)
	if len(parts) < 6 {
		return fmt.Errorf("could not parse job: %s", row)
	}

	for _, p := range parts[0:5] {
		if !cronExp.MatchString(p) {
			return fmt.Errorf("could not parse job: %s", row)
		}
	}

	keys := c.getKeys(parts[0:5])
	cmd := strings.Join(parts[5:], " ")
	for _, key := range keys {
		a, ok := c.jobs[key]
		if !ok {
			a = []string{}
		}
		a = append(a, cmd)
		m[key] = a
	}
	return nil
}

func (c *Cron) getKeys(parts []string) []string {
	out := []string{}
	var hasRange bool
	for i, part := range parts {
		if strings.Index(part, "-") >= 1 {
			hasRange = true
			r := c.getRange(part)
			for _, x := range r {
				parts[i] = x
				out = append(out, c.getKeys(parts)...)
			}
		} else if strings.Index(part, ",") >= 1 {
			hasRange = true
			s := strings.Split(part, ",")
			for _, x := range s {
				parts[i] = x
				out = append(out, c.getKeys(parts)...)
			}
		}
	}
	if !hasRange {
		out = append(out, strings.Join(parts, " "))
	}
	return out
}

func (c *Cron) getRange(s string) []string {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		lg.Printf("could not parse %s", s)
		return []string{}
	}
	start, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		lg.Printf("could not parse %s", s)
		return []string{}
	}
	end, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		lg.Printf("could not parse %s", s)
		return []string{}
	}
	if end <= start {
		lg.Printf("could not parse %s", s)
		return []string{}
	}
	out := make([]string, end-start+1)
	j := 0
	for i := start; i <= end; i++ {
		out[j] = fmt.Sprintf("%d", i)
		j++
	}
	return out
}

func (c *Cron) checkJobs(t time.Time) {
	keys := c.getPossibilities(t)
	for _, k := range keys {
		cmds, ok := c.jobs[k]
		if ok {
			for _, cmd := range cmds {
				c.out <- Message{
					Type:   COMMAND,
					Sender: "cron",
					UUID:   GetUUID(),
					Body:   cmd,
				}
			}
		}
	}
}

type now struct {
	Minute  int
	Hour    int
	Day     int
	Month   int
	Weekday int
}

func (c *Cron) getPossibilities(t time.Time) []string {
	n := now{
		Minute:  t.Minute(),
		Hour:    t.Hour(),
		Day:     t.Day(),
		Month:   int(t.Month()),
		Weekday: int(t.Weekday()),
	}
	tpl, _ := template.New("possibilites").Parse(`
* * * * *
{{.Minute}} * * * *
* {{.Hour}} * * *
* * {{.Day}} * *
* * * {{.Month}} *
* * * * {{.Weekday}}
{{.Minute}} {{.Hour}} * * *
{{.Minute}} * {{.Day}} * *
{{.Minute}} * * {{.Month}} *
{{.Minute}} * * * {{.Weekday}}
* {{.Hour}} {{.Day}} * *
* * {{.Day}} {{.Month}} *
* * * {{.Month}} {{.Weekday}}
* {{.Hour}} * {{.Month}} *
* {{.Hour}} * * {{.Weekday}}
* * {{.Day}} * {{.Weekday}}
* * {{.Day}} {{.Month}} {{.Weekday}}
* {{.Hour}} * {{.Month}} {{.Weekday}}
* {{.Hour}} {{.Day}} * {{.Weekday}}
* {{.Hour}} {{.Day}} {{.Month}} *
{{.Minute}} * * {{.Month}} {{.Weekday}}
{{.Minute}} {{.Hour}} * * {{.Weekday}}
{{.Minute}} {{.Hour}} {{.Day}} * *
{{.Minute}} * {{.Day}} * {{.Weekday}}
{{.Minute}} * {{.Day}} {{.Month}} *
{{.Minute}} {{.Hour}} * {{.Month}} *
{{.Minute}} {{.Hour}} {{.Day}} {{.Month}} *
{{.Minute}} {{.Hour}} {{.Day}} * {{.Weekday}}
{{.Minute}} {{.Hour}} * {{.Month}} {{.Weekday}}
{{.Minute}} * {{.Day}} {{.Month}} {{.Weekday}}
* {{.Minute}} {{.Hour}} {{.Day}} {{.Weekday}}
{{.Minute}} {{.Hour}} {{.Day}} {{.Month}} {{.Weekday}}
`)
	buf := bytes.Buffer{}
	tpl.Execute(&buf, n)
	return strings.Split(buf.String(), "\n")
}
