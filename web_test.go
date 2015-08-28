package gogadgets_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/cswank/gogadgets"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randString() string {
	b := make([]rune, 10)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

var _ = Describe("server", func() {
	var (
		port int
		addr string
		out  chan gogadgets.Message
		in   chan gogadgets.Message
		s    *gogadgets.Server
		lg   *fakeLogger
	)

	BeforeEach(func() {
		lg = &fakeLogger{}

		port = 1024 + rand.Intn(65535-1024)
		addr = fmt.Sprintf("http://localhost:%d/gadgets", port)

		s = gogadgets.NewServer("localhost", port, true, lg)

		in = make(chan gogadgets.Message)
		out = make(chan gogadgets.Message)
		go s.Start(out, in)
		out <- gogadgets.Message{
			Type:     gogadgets.UPDATE,
			Sender:   "lab led",
			Location: "lab",
			Name:     "led",
			Value: gogadgets.Value{
				Value:  true,
				Output: true,
			},
		}
		out <- gogadgets.Message{
			Type:     gogadgets.UPDATE,
			Sender:   "hall led",
			Location: "hall",
			Name:     "led",
			Value: gogadgets.Value{
				Value:  false,
				Output: false,
			},
		}
	})
	Describe("when all's good", func() {
		It("sends the status", func() {
			r, err := http.Get(addr)

			Expect(err).To(BeNil())
			defer r.Body.Close()

			Expect(r.StatusCode).To(Equal(http.StatusOK))
			msgs := map[string]gogadgets.Message{}
			dec := json.NewDecoder(r.Body)
			err = dec.Decode(&msgs)
			Expect(err).To(BeNil())
			Expect(len(msgs)).To(Equal(2))
			msg, ok := msgs["lab led"]
			Expect(ok).To(BeTrue())
			Expect(msg.Value.Value).To(BeTrue())

			msg, ok = msgs["hall led"]
			Expect(ok).To(BeTrue())
			Expect(msg.Value.Value).To(BeFalse())
		})
		It("accepts a message from the outside world", func() {
			msg := gogadgets.Message{
				Type:   gogadgets.COMMAND,
				Sender: "me",
				Body:   "turn on lab led",
			}

			buf := &bytes.Buffer{}
			enc := json.NewEncoder(buf)
			err := enc.Encode(&msg)
			Expect(err).To(BeNil())

			r, err := http.Post(addr, "application/json", buf)

			Expect(err).To(BeNil())
			Expect(r.StatusCode).To(Equal(http.StatusOK))
			Expect(r.StatusCode).To(Equal(http.StatusOK))

			m := <-in
			Expect(m.Body).To(Equal("turn on lab led"))
		})
	})
})
