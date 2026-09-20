// Command phase0-bridge demonstrates the Phase 0 IPC bridge + null platform
// host spike: create a stub window, decode an invoke that tries to spoof
// caller identity, and show that host identity wins.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/ipc"
	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/platform/null"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "phase0-bridge: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	host := null.New(platform.OSLinux)
	ctx := context.Background()
	if err := host.CreateWindow(ctx, platform.WindowSpec{
		ID:     "main",
		Title:  "Phase 0",
		Origin: domain.OriginPackagedLocal,
	}); err != nil {
		return err
	}

	fmt.Println("features:")
	for f, s := range host.Features() {
		fmt.Printf("  %-18s available=%v  %s\n", f, s.Available, s.Detail)
	}

	raw, err := json.Marshal(ipc.Envelope{
		Protocol: ipc.ProtocolVersion,
		Kind:     ipc.KindInvoke,
		ID:       "spike-1",
		Payload: mustJSON(ipc.InvokePayload{
			Command:       "project.open",
			ClaimedWindow: "admin",
			ClaimedOrigin: "https://evil.example",
			ResourcePath:  "/project/app",
		}),
	})
	if err != nil {
		return err
	}

	bridge := ipc.Bridge{Host: ipc.HostIdentity{
		Window: "main",
		Origin: domain.OriginPackagedLocal,
	}}
	req, id, err := bridge.DecodeInvoke(raw)
	if err != nil {
		return err
	}
	fmt.Printf("decoded id=%s command=%s caller={window:%s origin:%s}\n",
		id, req.Command, req.Caller.Window, req.Caller.Origin)
	fmt.Println("(spoofed admin / evil.example were discarded)")

	if err := host.DialogOpen(ctx); err != nil {
		fmt.Printf("dialog.open → %v\n", err)
	}
	return nil
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
