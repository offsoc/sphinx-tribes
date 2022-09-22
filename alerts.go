package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
)

type Action struct {
	Action   string `json:"action"`    // "dm"
	ChatUuid string `json:"chat_uuid"` // tribe uuid
	Pubkey   string `json:"pubkey"`
	Content  string `json:"content"`
}

func processAlerts(p Person) {
	// get the existing person's github_issues
	// compare to see if there are new ones
	// check the new ones for coding languages like "#Javascript"

	// people need a new "extras"->"alerts" that lets them toggle on and off alerts
	// pull people who have alerts on
	// of those people, who have "Javascript" in their "coding_languages"?
	// if they match, build an Action with their pubkey
	// post all the Actions you have build to relay with the HMAC header

	relayUrl := os.Getenv("ALERT_URL")
	alertSecret := os.Getenv("ALERT_SECRET")
	alertTribeUuid := os.Getenv("ALERT_TRIBE_UUID")
	if relayUrl == "" || alertSecret == "" || alertTribeUuid == "" {
		fmt.Println("Ticket alerts: ENV information not found")
		return
	}

	var action Action
	action.ChatUuid = alertTribeUuid
	action.Action = "dm"
	action.Content = "A new ticket relevant to your interests has been created on Sphinx Community - https://community.sphinx.chat/p?owner_id="
	action.Content += p.OwnerPubKey
	action.Content += "&created=" + strconv.Itoa(int(p.newTicketTime))

	// Check that new ticket time exists
	if p.newTicketTime == 0 {
		fmt.Println("Ticket alerts: New ticket time not found")
		return
	}

	var issue PropertyMap = nil
	wanteds, ok := p.Extras["wanted"].([]interface{})
	if !ok {
		fmt.Println("Ticket alerts: No tickets found for person")
	}
	for _, wanted := range wanteds {
		w, ok2 := wanted.(map[string]interface{})
		if !ok2 {
			continue
		}
		time, ok3 := w["created"].(int64)
		if !ok3 {
			continue
		}
		if time == p.newTicketTime {
			issue = w
			break
		}
	}

	if issue == nil {
		fmt.Println("Ticket alerts: No ticket identified with the correct timestamp")
	}

	languages, ok4 := issue["codingLanguage"].([]interface{})
	if !ok4 {
		fmt.Println("Ticket alerts: No languages found in ticket")
		return
	}

	var err error
	people, err := DB.getPeopleForNewTicket(languages)
	if err != nil {
		fmt.Println("Ticket alerts: DB query to get interested people failed", err)
		return
	}

	client := http.Client{}

	for _, per := range people {
		action.Pubkey = per.OwnerPubKey
		var buf bytes.Buffer
		err = json.NewEncoder(&buf).Encode(action)
		if err != nil {
			fmt.Println("Ticket alerts: Unable to parse message into byte buffer", err)
			return
		}
		request, err := http.NewRequest("POST", relayUrl, &buf)
		if err != nil {
			fmt.Println("Ticket alerts: Unable to create a request to send to relay", err)
			return
		}

		b := buf.Bytes()

		secret, err := hex.DecodeString(alertSecret)
		if err != nil {
			fmt.Println("Ticket alerts: Unable to create a byte array for secret", err)
			return
		}

		mac := hmac.New(sha256.New, secret)
		mac.Write(b)
		hmac256Byte := mac.Sum(nil)
		hmac256Hex := "sha256=" + hex.EncodeToString(hmac256Byte)

		request.Header.Set("x-hub-signature-256", hmac256Hex)
		_, err = client.Do(request)
		if err != nil {
			fmt.Println("Ticket alerts: Unable to communicate request to relay", err)
		}
	}

	return
}
