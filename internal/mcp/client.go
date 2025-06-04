// Package: internal/mcp/client.go
package mcp

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Client struct {
	conn       net.Conn
	encoder    *json.Encoder
	decoder    *json.Decoder
	requestID  int64
	responses  map[interface{}]chan MCPResponse
	mu         sync.RWMutex
	serverInfo ServerInfo
}

func NewMCPClient() *Client {
	return &Client{
		responses: make(map[interface{}]chan MCPResponse),
	}
}

func (c *Client) ConnectUnix(socketPath string) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to unix socket: %w", err)
	}

	c.conn = conn
	c.encoder = json.NewEncoder(conn)
	c.decoder = json.NewDecoder(conn)

	go c.readResponses()
	return nil
}

func (c *Client) ConnectTCP(host string, port int) error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return fmt.Errorf("failed to connect to TCP: %w", err)
	}

	c.conn = conn
	c.encoder = json.NewEncoder(conn)
	c.decoder = json.NewDecoder(conn)

	go c.readResponses()
	return nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) Initialize(clientName, clientVersion string) error {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      c.nextRequestID(),
		Method:  "initialize",
		Params: InitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities: ClientCapabilities{
				Roots:    true,
				Sampling: false,
			},
			ClientInfo: ClientInfo{
				Name:    clientName,
				Version: clientVersion,
			},
		},
	}

	resp, err := c.sendRequest(req)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize failed: %s", resp.Error.Message)
	}

	var result InitializeResult
	resultJSON, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)

	c.serverInfo = result.ServerInfo
	return nil
}

func (c *Client) ListTools() ([]MCPTool, error) {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      c.nextRequestID(),
		Method:  "tools/list",
	}

	resp, err := c.sendRequest(req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("list tools failed: %s", resp.Error.Message)
	}

	var result struct {
		Tools []MCPTool `json:"tools"`
	}
	resultJSON, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)

	return result.Tools, nil
}

func (c *Client) CallTool(name string, arguments map[string]interface{}) (string, error) {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      c.nextRequestID(),
		Method:  "tools/call",
		Params: CallToolParams{
			Name:      name,
			Arguments: arguments,
		},
	}

	resp, err := c.sendRequest(req)
	if err != nil {
		return "", err
	}

	if resp.Error != nil {
		return "", fmt.Errorf("tool call failed: %s", resp.Error.Message)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	resultJSON, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)

	if len(result.Content) > 0 && result.Content[0].Type == "text" {
		return result.Content[0].Text, nil
	}

	return "", fmt.Errorf("unexpected response format")
}

func (c *Client) sendRequest(req MCPRequest) (MCPResponse, error) {
	respChan := make(chan MCPResponse, 1)

	c.mu.Lock()
	c.responses[req.ID] = respChan
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.responses, req.ID)
		c.mu.Unlock()
	}()

	if err := c.encoder.Encode(req); err != nil {
		return MCPResponse{}, fmt.Errorf("failed to send request: %w", err)
	}

	select {
	case resp := <-respChan:
		return resp, nil
	case <-time.After(30 * time.Second):
		return MCPResponse{}, fmt.Errorf("request timeout")
	}
}

func (c *Client) readResponses() {
	for {
		var resp MCPResponse
		if err := c.decoder.Decode(&resp); err != nil {
			return // Connection closed
		}

		c.mu.RLock()
		if respChan, exists := c.responses[resp.ID]; exists {
			select {
			case respChan <- resp:
			default:
			}
		}
		c.mu.RUnlock()
	}
}

func (c *Client) nextRequestID() int64 {
	return atomic.AddInt64(&c.requestID, 1)
}
