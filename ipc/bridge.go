// Package ipc defines Vitra's Phase 0 frontend↔runtime message protocol.
//
// The codec never trusts caller identity fields inside the payload. Adapters
// must supply WindowID and Origin from the native WebView host when decoding
// inbound invokes (security invariants 3 and 4).
package ipc

import (
	"encoding/json"
	"errors"
	"fmt"

	"go.klarlabs.de/vitra/domain"
)

// ProtocolVersion is the wire protocol version carried on every envelope.
// Generated frontend bindings must match this version (reliability invariant 8).
const ProtocolVersion = "1"

// Kind classifies envelope payloads.
type Kind string

const (
	KindInvoke       Kind = "invoke"
	KindInvokeResult Kind = "invoke_result"
	KindEvent        Kind = "event"
	KindError        Kind = "error"
)

// Envelope is the versioned wire wrapper for all IPC messages.
type Envelope struct {
	Protocol string          `json:"protocol"`
	Kind     Kind            `json:"kind"`
	ID       string          `json:"id,omitempty"`
	Payload  json.RawMessage `json:"payload"`
}

// InvokePayload is the untrusted frontend payload for a command invoke.
// Window/Origin fields MUST be ignored by the bridge — they exist only so
// tests can prove spoofed identity is discarded.
type InvokePayload struct {
	Command      string          `json:"command"`
	Input        json.RawMessage `json:"input,omitempty"`
	ResourcePath string          `json:"resource_path,omitempty"`
	// Spoofable identity — never trusted.
	ClaimedWindow string `json:"window,omitempty"`
	ClaimedOrigin string `json:"origin,omitempty"`
}

// ResultPayload is a successful invoke response.
type ResultPayload struct {
	Command string          `json:"command"`
	Output  json.RawMessage `json:"output,omitempty"`
}

// ErrorPayload is a structured IPC error (including capability denials).
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HostIdentity is caller identity established at the native boundary.
type HostIdentity struct {
	Window domain.WindowID
	Origin domain.Origin
}

// Bridge decodes inbound envelopes using host-supplied identity.
type Bridge struct {
	Host HostIdentity
}

// DecodeInvoke validates protocol version and builds a domain InvocationRequest
// using Host identity, discarding any claimed window/origin in the payload.
func (b Bridge) DecodeInvoke(raw []byte) (domain.InvocationRequest, string, error) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return domain.InvocationRequest{}, "", fmt.Errorf("ipc decode: %w", err)
	}
	if env.Protocol != ProtocolVersion {
		return domain.InvocationRequest{}, "", fmt.Errorf("unsupported ipc protocol %q (want %q)", env.Protocol, ProtocolVersion)
	}
	if env.Kind != KindInvoke {
		return domain.InvocationRequest{}, "", fmt.Errorf("expected kind %q, got %q", KindInvoke, env.Kind)
	}
	var payload InvokePayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return domain.InvocationRequest{}, "", fmt.Errorf("invoke payload: %w", err)
	}
	if payload.Command == "" {
		return domain.InvocationRequest{}, "", errors.New("command is required")
	}
	caller, err := domain.NewCaller(b.Host.Window, b.Host.Origin)
	if err != nil {
		return domain.InvocationRequest{}, "", err
	}
	var input any
	if len(payload.Input) > 0 && string(payload.Input) != "null" {
		if err := json.Unmarshal(payload.Input, &input); err != nil {
			return domain.InvocationRequest{}, "", fmt.Errorf("input: %w", err)
		}
	}
	return domain.InvocationRequest{
		Caller:       caller,
		Command:      domain.CommandName(payload.Command),
		Input:        input,
		ResourcePath: payload.ResourcePath,
	}, env.ID, nil
}

// EncodeResult encodes a successful invoke result envelope.
func EncodeResult(id string, command domain.CommandName, output any) ([]byte, error) {
	out, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(ResultPayload{Command: string(command), Output: out})
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{
		Protocol: ProtocolVersion,
		Kind:     KindInvokeResult,
		ID:       id,
		Payload:  payload,
	})
}

// EncodeError encodes a structured error envelope.
func EncodeError(id string, code, message string) ([]byte, error) {
	payload, err := json.Marshal(ErrorPayload{Code: code, Message: message})
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{
		Protocol: ProtocolVersion,
		Kind:     KindError,
		ID:       id,
		Payload:  payload,
	})
}

// DenialCode maps a domain denial to a stable IPC error code.
func DenialCode(err error) string {
	var denied *domain.ErrDenied
	if errors.As(err, &denied) {
		return string(denied.Code)
	}
	return "error"
}
