// Package mcp serves a small Model Context Protocol endpoint on stdio.
// Cursor launches `ttcli mcp` and talks JSON-RPC. Tools can search open
// tasks, read one task, and change its title and notes.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const protocolVersion = "2024-11-05"

type rpcRequest struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// Run reads newline-delimited JSON-RPC from in and writes responses to out.
// Logging stays off stdout so it cannot break the protocol stream.
func Run(ctx context.Context, api TaskAPI, in io.Reader, out io.Writer) error {
	if api == nil {
		return fmt.Errorf("mcp task api unavailable")
	}
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			_ = writeError(enc, nil, -32700, "parse error")
			continue
		}
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue
		}
		if err := handle(enc, api, req); err != nil {
			return err
		}
	}
	return sc.Err()
}

func handle(enc *json.Encoder, api TaskAPI, req rpcRequest) error {
	switch req.Method {
	case "initialize":
		version := protocolVersion
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if json.Unmarshal(req.Params, &params) == nil && strings.HasPrefix(params.ProtocolVersion, "202") {
			version = params.ProtocolVersion
		}
		return writeResult(enc, req.ID, map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "ttcli", "version": "0.1.0"},
		})
	case "ping":
		return writeResult(enc, req.ID, map[string]any{})
	case "tools/list":
		return writeResult(enc, req.ID, map[string]any{"tools": toolDefs()})
	case "tools/call":
		result := callTool(api, req.Params)
		return writeResult(enc, req.ID, result)
	default:
		return writeError(enc, req.ID, -32601, "method not found")
	}
}

func callTool(api TaskAPI, params json.RawMessage) toolResult {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return toolError(err.Error())
	}
	if len(call.Arguments) == 0 {
		call.Arguments = []byte(`{}`)
	}
	var text string
	var err error
	switch call.Name {
	case "search_tasks":
		text, err = searchTool(api, call.Arguments)
	case "get_task":
		text, err = getTool(api, call.Arguments)
	case "update_task_text":
		text, err = updateTool(api, call.Arguments)
	default:
		err = fmt.Errorf("unknown tool %q", call.Name)
	}
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult{Content: []toolContent{{Type: "text", Text: text}}}
}

func toolError(message string) toolResult {
	return toolResult{
		IsError: true,
		Content: []toolContent{{Type: "text", Text: message}},
	}
}

func writeResult(enc *json.Encoder, id json.RawMessage, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return enc.Encode(rpcResponse{JSONRPC: "2.0", ID: id, Result: raw})
}

func writeError(enc *json.Encoder, id json.RawMessage, code int, message string) error {
	if len(id) == 0 {
		id = []byte("null")
	}
	return enc.Encode(rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}
