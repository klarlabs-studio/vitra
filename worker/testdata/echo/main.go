// Echo worker for StdioIPC tests: JSON-line req → res with same payload.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type frame struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Method  string          `json:"method,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	for in.Scan() {
		var f frame
		if err := json.Unmarshal(in.Bytes(), &f); err != nil {
			os.Exit(2)
		}
		switch f.Type {
		case "req":
			if f.Method == "echo" {
				resp := frame{ID: f.ID, Type: "res", Payload: f.Payload}
				b, _ := json.Marshal(resp)
				fmt.Fprintf(out, "%s\n", b)
				_ = out.Flush()
				continue
			}
			resp := frame{ID: f.ID, Type: "err", Error: "unknown method"}
			b, _ := json.Marshal(resp)
			fmt.Fprintf(out, "%s\n", b)
			_ = out.Flush()
		case "msg":
			// ignore
		default:
			os.Exit(3)
		}
	}
	if err := in.Err(); err != nil {
		os.Exit(1)
	}
}
