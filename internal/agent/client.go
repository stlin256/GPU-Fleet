package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gpufleet/internal/auth"
	"gpufleet/internal/model"
)

type Client struct {
	ServerURL string
	DeviceID  string
	Secret    string
	Timeout   time.Duration
	UseGzip   bool
	HTTP      *http.Client
}

func (c *Client) PostHeartbeat(heartbeat model.Heartbeat) error {
	return c.postJSON("/api/v1/agent/heartbeat", heartbeat)
}

func (c *Client) PostSamples(batch model.SampleBatch) error {
	return c.postJSON("/api/v1/agent/samples", batch)
}

func (c *Client) PostProcesses(batch model.ProcessBatch) error {
	return c.postJSON("/api/v1/agent/process-snapshots", batch)
}

func (c *Client) PostConfig(report model.AgentConfigReport) error {
	return c.postJSON("/api/v1/agent/config", report)
}

func (c *Client) GetUpdatePolicy() (model.AgentUpdatePolicy, error) {
	var response struct {
		Policy model.AgentUpdatePolicy `json:"policy"`
	}
	if err := c.postJSONDecode("/api/v1/agent/update-policy", map[string]string{
		"agent_version": model.AgentVersion,
	}, &response); err != nil {
		return model.AgentUpdatePolicy{}, err
	}
	return response.Policy, nil
}

func (c *Client) PostUpdateEvent(event model.AgentUpdateEvent) error {
	return c.postJSON("/api/v1/agent/update-events", event)
}

func (c *Client) postJSON(path string, value any) error {
	return c.postJSONDecode(path, value, nil)
}

func (c *Client) postJSONDecode(path string, value any, out any) error {
	if c.Timeout == 0 {
		c.Timeout = 10 * time.Second
	}
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	requestBody := body
	var reader io.Reader = bytes.NewReader(requestBody)
	contentEncoding := ""
	if c.UseGzip {
		var compressed bytes.Buffer
		gw := gzip.NewWriter(&compressed)
		if _, err := gw.Write(body); err != nil {
			_ = gw.Close()
			return err
		}
		if err := gw.Close(); err != nil {
			return err
		}
		requestBody = compressed.Bytes()
		reader = bytes.NewReader(requestBody)
		contentEncoding = "gzip"
	}

	endpoint, err := joinURL(c.ServerURL, path)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if contentEncoding != "" {
		req.Header.Set("Content-Encoding", contentEncoding)
	}
	if err := auth.AttachSignedHeaders(req, body, c.DeviceID, c.Secret, time.Now().UTC()); err != nil {
		return err
	}

	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: c.Timeout}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("server returned %s: %s", res.Status, strings.TrimSpace(string(limited)))
	}
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			return err
		}
	}
	return nil
}

func joinURL(base, path string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	return parsed.String(), nil
}
