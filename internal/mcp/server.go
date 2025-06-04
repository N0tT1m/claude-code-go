// Package: internal/mcp/server.go
package mcp

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/N0tT1m/claude-code-go/internal/tools"
)

// MCP (Model Context Protocol) implementation
type Server struct {
	name         string
	version      string
	tools        *tools.Registry
	resources    map[string]Resource
	listeners    []net.Listener
	mu           sync.RWMutex
	capabilities ServerCapabilities
}

type ServerCapabilities struct {
	Tools     bool `json:"tools"`
	Resources bool `json:"resources"`
	Prompts   bool `json:"prompts"`
}

type Resource struct {
	URI         string            `json:"uri"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	MimeType    string            `json:"mimeType"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
}

type ClientCapabilities struct {
	Roots    bool `json:"roots"`
	Sampling bool `json:"sampling"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func NewMCPServer(name, version string, toolRegistry *tools.Registry) *Server {
	return &Server{
		name:      name,
		version:   version,
		tools:     toolRegistry,
		resources: make(map[string]Resource),
		capabilities: ServerCapabilities{
			Tools:     true,
			Resources: true,
			Prompts:   false,
		},
	}
}

func (s *Server) Start(socketPath string) error {
	// Clean up existing socket
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create unix socket: %w", err)
	}

	s.mu.Lock()
	s.listeners = append(s.listeners, listener)
	s.mu.Unlock()

	go s.acceptConnections(listener)
	return nil
}

func (s *Server) StartTCP(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to create TCP listener: %w", err)
	}

	s.mu.Lock()
	s.listeners = append(s.listeners, listener)
	s.mu.Unlock()

	go s.acceptConnections(listener)
	return nil
}

func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, listener := range s.listeners {
		listener.Close()
	}
	s.listeners = nil
	return nil
}

func (s *Server) acceptConnections(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return // Listener closed
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var req MCPRequest
		if err := decoder.Decode(&req); err != nil {
			return // Connection closed or malformed JSON
		}

		resp := s.handleRequest(req)
		if err := encoder.Encode(resp); err != nil {
			return // Failed to send response
		}
	}
}

func (s *Server) handleRequest(req MCPRequest) MCPResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleListTools(req)
	case "tools/call":
		return s.handleCallTool(req)
	case "resources/list":
		return s.handleListResources(req)
	case "resources/read":
		return s.handleReadResource(req)
	default:
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

func (s *Server) handleInitialize(req MCPRequest) MCPResponse {
	var params InitializeParams
	if paramsData, ok := req.Params.(map[string]interface{}); ok {
		paramsJSON, _ := json.Marshal(paramsData)
		json.Unmarshal(paramsJSON, &params)
	}

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    s.capabilities,
		ServerInfo: ServerInfo{
			Name:    s.name,
			Version: s.version,
		},
	}

	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleListTools(req MCPRequest) MCPResponse {
	tools := s.tools.GetAvailable()

	mcpTools := make([]MCPTool, len(tools))
	for i, tool := range tools {
		mcpTools[i] = MCPTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.Parameters,
		}
	}

	result := map[string]interface{}{
		"tools": mcpTools,
	}

	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

type MCPTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

func (s *Server) handleCallTool(req MCPRequest) MCPResponse {
	var params CallToolParams
	if paramsData, ok := req.Params.(map[string]interface{}); ok {
		paramsJSON, _ := json.Marshal(paramsData)
		json.Unmarshal(paramsJSON, &params)
	}

	result, err := s.tools.Execute(params.Name, params.Arguments)
	if err != nil {
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: fmt.Sprintf("Tool execution failed: %s", err.Error()),
			},
		}
	}

	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": result,
				},
			},
		},
	}
}

func (s *Server) handleListResources(req MCPRequest) MCPResponse {
	s.mu.RLock()
	resources := make([]Resource, 0, len(s.resources))
	for _, resource := range s.resources {
		resources = append(resources, resource)
	}
	s.mu.RUnlock()

	result := map[string]interface{}{
		"resources": resources,
	}

	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

type ReadResourceParams struct {
	URI string `json:"uri"`
}

func (s *Server) handleReadResource(req MCPRequest) MCPResponse {
	var params ReadResourceParams
	if paramsData, ok := req.Params.(map[string]interface{}); ok {
		paramsJSON, _ := json.Marshal(paramsData)
		json.Unmarshal(paramsJSON, &params)
	}

	s.mu.RLock()
	resource, exists := s.resources[params.URI]
	s.mu.RUnlock()

	if !exists {
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Resource not found",
			},
		}
	}

	// For file resources, read the content
	if resource.MimeType == "text/plain" || resource.MimeType == "application/octet-stream" {
		content, err := os.ReadFile(resource.URI)
		if err != nil {
			return MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &MCPError{
					Code:    -32602,
					Message: fmt.Sprintf("Failed to read resource: %s", err.Error()),
				},
			}
		}

		result := map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"uri":      resource.URI,
					"mimeType": resource.MimeType,
					"text":     string(content),
				},
			},
		}

		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}
	}

	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error: &MCPError{
			Code:    -32602,
			Message: "Unsupported resource type",
		},
	}
}

func (s *Server) RegisterResource(uri, name, description, mimeType string, metadata map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resources[uri] = Resource{
		URI:         uri,
		Name:        name,
		Description: description,
		MimeType:    mimeType,
		Metadata:    metadata,
	}
}
