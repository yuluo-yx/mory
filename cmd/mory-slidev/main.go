package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/yuluo-yx/mory/internal/slidevexport"
)

type response struct {
	OK    bool   `json:"ok"`
	Code  string `json:"code,omitempty"`
	Error string `json:"error,omitempty"`
}

func main() {
	var request slidevexport.Request
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		writeResponse(response{Code: slidevexport.CodeFailed, Error: fmt.Sprintf("decode Slidev request: %v", err)})
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := slidevexport.Export(ctx, request); err != nil {
		writeResponse(response{Code: slidevexport.ErrorCode(err), Error: err.Error()})
		os.Exit(1)
	}
	writeResponse(response{OK: true})
}

func writeResponse(value response) {
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}
}
