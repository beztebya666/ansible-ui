// Package runnerclient is the api's WebSocket client to the runner service.
package runnerclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nikiv/ansible-ui/internal/model"
)

// SyncGit asks the runner to clone/update a repository (HTTP, not WS).
func SyncGit(ctx context.Context, baseHTTP string, spec model.GitSyncSpec) (*model.GitSyncResult, error) {
	body, _ := json.Marshal(spec)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseHTTP+"/v1/git/sync", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("runner git sync: HTTP %d", resp.StatusCode)
	}
	var res model.GitSyncResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

// CommitGit asks the runner to commit files onto a new branch and push it.
func CommitGit(ctx context.Context, baseHTTP string, spec model.GitCommitSpec) (*model.GitCommitResult, error) {
	body, _ := json.Marshal(spec)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseHTTP+"/v1/git/commit", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("runner git commit: HTTP %d", resp.StatusCode)
	}
	var res model.GitCommitResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

// ListInventory asks the runner to resolve an inventory's hosts/groups (HTTP).
func ListInventory(ctx context.Context, baseHTTP string, spec model.InventoryListSpec) (*model.InventoryListResult, error) {
	body, _ := json.Marshal(spec)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseHTTP+"/v1/inventory/list", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("runner inventory list: HTTP %d", resp.StatusCode)
	}
	var res model.InventoryListResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

// GatherFacts asks the runner to gather Ansible facts for an inventory (HTTP).
func GatherFacts(ctx context.Context, baseHTTP string, spec model.FactsGatherSpec) (*model.FactsGatherResult, error) {
	body, _ := json.Marshal(spec)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseHTTP+"/v1/facts/gather", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("runner facts gather: HTTP %d", resp.StatusCode)
	}
	var res model.FactsGatherResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

// PingHosts asks the runner to check reachability of an inventory's hosts (HTTP).
func PingHosts(ctx context.Context, baseHTTP string, spec model.PingSpec) (*model.PingResult, error) {
	body, _ := json.Marshal(spec)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseHTTP+"/v1/host/ping", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("runner host ping: HTTP %d", resp.StatusCode)
	}
	var res model.PingResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

// AnsibleVersion returns the runner's `ansible --version` output (HTTP).
func AnsibleVersion(ctx context.Context, baseHTTP string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseHTTP+"/v1/ansible/version", nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("runner ansible version: HTTP %d", resp.StatusCode)
	}
	var res struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Version, nil
}

// Conn is a single exec stream to the runner.
type Conn struct {
	ws *websocket.Conn
	mu sync.Mutex
}

// Dial opens an exec stream and sends the ExecSpec as the first message.
func Dial(ctx context.Context, baseURL string, spec model.ExecSpec) (*Conn, error) {
	d := websocket.Dialer{HandshakeTimeout: 15 * time.Second, WriteBufferSize: 64 * 1024}
	ws, _, err := d.DialContext(ctx, baseURL+"/v1/exec", nil)
	if err != nil {
		return nil, err
	}
	if err := ws.WriteJSON(spec); err != nil {
		_ = ws.Close()
		return nil, err
	}
	return &Conn{ws: ws}, nil
}

// Recv blocks for the next frame from the runner.
func (c *Conn) Recv() (model.Frame, error) {
	var f model.Frame
	err := c.ws.ReadJSON(&f)
	return f, err
}

// Send forwards a control frame (resize/cancel) to the runner.
func (c *Conn) Send(f model.Frame) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.ws.WriteJSON(f)
}

// Close tears down the stream.
func (c *Conn) Close() error { return c.ws.Close() }
