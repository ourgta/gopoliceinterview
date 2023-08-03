package main

import (
	"errors"
	"fmt"
	"gopoliceinterview/internal/discord"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/exp/slices"
)

func main() {
	serverId := os.Getenv("SERVER_ID")
	channelId := os.Getenv("CHANNEL_ID")
	t := os.Getenv("TIMEOUT")
	timeout, err := strconv.Atoi(t)
	if err != nil {
		log.Fatal(err)
	}

	webhook := os.Getenv("WEBHOOK")
	session := discord.Session{
		Webhook: webhook,
	}

	var clients []string
	var last time.Time
	init := false

	for {
		time.Sleep(time.Until(last.Add(time.Duration(timeout) * time.Minute)))

		doc, err := func() (doc *goquery.Document, err error) {
			resp, err := http.Get("https://www.tsviewer.com/ts3viewer.php?ID=" + url.QueryEscape(serverId))
			if err != nil {
				return
			}

			last = time.Now()

			defer func(resp *http.Response) {
				err := resp.Body.Close()
				if err != nil {
					log.Println(err)
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
				after, found := strings.CutPrefix(line, "TSV.ViewerScript.Data["+serverId+"]['html'] = ")
				if !found {
					continue
				}

				after, found = strings.CutSuffix(after, "';")
				if !found {
					continue
				}

				html = strings.ReplaceAll(after, "\\\"", "\"")
			}

			if html == "" {
				err = errors.New("no html")
				return
			}

			doc, err = goquery.NewDocumentFromReader(strings.NewReader(html))
			return
		}()

		if err != nil {
			log.Println(err)
			continue
		}

		var newClients []string
		var message string
		doc.Find("div.tsv_user").Each(func(_ int, s *goquery.Selection) {
			id, exists := s.Attr("data-cid")
			if !exists {
				return
			}

			if id != channelId {
				return
			}

			name, _ := s.Attr("data-client_nickname")
			newClients = append(newClients, name)
			if !init {
				return
			}

			if slices.Contains(clients, name) {
				return
			}

			if message != "" {
				message += "\n"
			}

			message += fmt.Sprintf("**%v** is waiting for an interview", name)
		})

		clients = newClients
		init = true

		if message == "" {
			continue
		}

		err = session.Message(message)
		if err != nil {
			log.Println(err)
		}
	}
}
