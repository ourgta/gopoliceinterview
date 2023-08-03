package discord

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
)

type Session struct {
	Webhook string
	last    time.Time
}

func (session *Session) Message(content string) (err error) {
	time.Sleep(time.Until(session.last.Add(2 * time.Second)))

	buf, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		return
	}

	resp, err := http.Post(session.Webhook, "application/json", bytes.NewBuffer(buf))
	if err != nil {
		return
	}

	session.last = time.Now()

	func(resp *http.Response) {
		err := resp.Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(resp)

	if resp.StatusCode != 204 {
		err = errors.New(resp.Status)
	}

	return
}
