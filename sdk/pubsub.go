package sdk

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	BaseURL string
}

type Message struct {
	ID      int    `json:"id"`
	Topic   string `json:"topic"`
	Content string `json:"content"`
}

// NewClient returns a new Pub/Sub client
func NewClient(baseURL string) *Client {
	return &Client{BaseURL: baseURL}
}

// Publish sends a message to a topic
func (c *Client) Publish(topic, content string) (*Message, error) {
	msg := Message{Topic: topic, Content: content}
	body, _ := json.Marshal(msg)

	resp, err := http.Post(c.BaseURL+"/publish", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result Message
	err = json.NewDecoder(resp.Body).Decode(&result)
	return &result, err
}

// Subscribe connects to the server via SSE and invokes the callback on each message
func (c *Client) Subscribe(topic string, onMessage func(Message)) error {
	url := fmt.Sprintf("%s/subscribe?topic=%s", c.BaseURL, topic)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	go func() {
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Println("Read error:", err)
				}
				break
			}
			if bytes.HasPrefix(line, []byte("data: ")) {
				var msg Message
				json.Unmarshal(line[6:], &msg)
				onMessage(msg)
			}
		}
	}()

	return nil
}
