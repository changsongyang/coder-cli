package client

import (
	"context"
	"net/url"
	"time"

	"nhooyr.io/websocket"
)

type Environment struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

func (c Client) Envs(user *User, org Org) ([]Environment, error) {
	var envs []Environment
	err := c.requestBody(
		"GET", "/api/environments/?user_id="+user.ID+"&organization_id="+org.ID,
		nil,
		&envs,
	)
	return envs, err
}

func (c Client) Wush(env Environment, cmd string, args ...string) (*websocket.Conn, error) {
	u := c.copyURL()
	if c.BaseURL.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = "/proxy/environments/" + env.ID + "/wush-lite"
	query := make(url.Values)
	query.Set("command", cmd)
	query["args[]"] = args
	query.Set("tty", "false")
	query.Set("stdin", "true")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	fullURL := u.String() + "?" + query.Encode()

	conn, resp, err := websocket.Dial(ctx, fullURL,
		&websocket.DialOptions{
			HTTPHeader: map[string][]string{
				"Cookie": {"session_token=" + c.Token},
			},
		},
	)
	if err != nil {
		if resp != nil {
			return nil, bodyError(resp)
		}
		return nil, err
	}
	return conn, nil
}
