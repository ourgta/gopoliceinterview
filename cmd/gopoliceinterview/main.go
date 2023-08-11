package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopoliceinterview/internal/discord"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func main() {
	var configs []struct {
		ServerID      int    `json:"serverId"`
		ChannelID     int    `json:"channelId"`
		WebhookURL    string `json:"webhookUrl"`
		MessageSuffix string `json:"messageSuffix"`
	}

	if contents, err := os.ReadFile("config.json"); err != nil {
		log.Fatal("readfile:", err)
	} else {
		if err := json.Unmarshal(contents, &configs); err != nil {
			log.Fatal("unmarshal:", err)
		}
	}

	servers := map[int][]struct {
		ChannelID     int
		WebhookURL    string
		MessageSuffix string
	}{}

	for _, config := range configs {
		servers[config.ServerID] = append(servers[config.ServerID], struct {
			ChannelID     int
			WebhookURL    string
			MessageSuffix string
		}{
			config.ChannelID,
			config.WebhookURL,
			config.MessageSuffix,
		})
	}

	t := os.Getenv("TIMEOUT")
	timeout, err := strconv.Atoi(t)
	if err != nil {
		log.Fatal(err)
	}

	var (
		clients = map[int]map[int][]string{}
		session = discord.Session{}
		last    time.Time
		init    bool
	)

	for {
		if !last.IsZero() {
			time.Sleep(time.Until(last.Add(time.Duration(timeout) * time.Minute)))
		}

		var (
			newClients = map[int]map[int][]string{}
			messages   = map[string]string{}
		)

		for server, channels := range servers {
			doc, err := func() (doc *goquery.Document, err error) {
				resp, err := http.Get(fmt.Sprintf("https://www.tsviewer.com/ts3viewer.php?ID=%v", server))
				if err != nil {
					return
				}

				last = time.Now()

				defer func(resp *http.Response) {
					err := resp.Body.Close()
					if err != nil {
						log.Println("close:", err)
					}
				}(resp)

				if resp.StatusCode != 200 {
					err = errors.New(resp.Status)
					return
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return
				}

				var html string
				for _, line := range strings.Split(string(body), "\n") {
					after, found := strings.CutPrefix(line, fmt.Sprintf("TSV.ViewerScript.Data[%v]['html'] = ", server))
					if !found {
						continue
					}

					before, found := strings.CutSuffix(after, "';")
					if !found {
						continue
					}

					html = strings.ReplaceAll(before, "\\\"", "\"")
				}

				if html == "" {
					err = errors.New("no html")
					return
				}

				doc, err = goquery.NewDocumentFromReader(strings.NewReader(html))
				return
			}()

			if err != nil {
				log.Println("document:", err)
				continue
			}

			doc.Find("div.tsv_user").Each(func(_ int, s *goquery.Selection) {
				var id int
				if val, exists := s.Attr("data-cid"); !exists {
					return
				} else {
					if id, err = strconv.Atoi(val); err != nil {
						return
					}
				}

				for _, channel := range channels {
					if id != channel.ChannelID {
						continue
					}

					name, _ := s.Attr("data-client_nickname")
					if newClients[server] == nil {
						newClients[server] = map[int][]string{}
					}

					newClients[server][id] = append(newClients[server][id], name)
					if !init {
						continue
					}

					if slices.Contains(clients[server][id], name) {
						continue
					}

					if messages[channel.WebhookURL] != "" {
						messages[channel.WebhookURL] += "\n"
					}

					messages[channel.WebhookURL] += fmt.Sprintf("**%v**%v", name, channel.MessageSuffix)
				}
			})
		}

		for webhookURL, message := range messages {
			if message == "" {
				continue
			}

			session.Webhook = webhookURL
			err = session.Message(message)
			if err != nil {
				log.Println("message", err)
			}
		}

		clients = newClients
		init = true
	}
}
