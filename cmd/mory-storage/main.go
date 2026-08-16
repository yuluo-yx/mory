package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/yuluo-yx/mory/internal/storage"
)

type request struct {
	Action    string         `json:"action"`
	Root      string         `json:"root"`
	Workspace storage.Config `json:"workspace"`
}

type response struct {
	OK      bool            `json:"ok"`
	Summary storage.Summary `json:"summary,omitempty"`
	Error   string          `json:"error,omitempty"`
}

func main() {
	if err := run(); err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(response{OK: false, Error: err.Error()})
		os.Exit(1)
	}
}

func run() error {
	var input request
	decoder := json.NewDecoder(os.Stdin)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	if input.Root == "" {
		return errors.New("workspace root is required")
	}

	backend, err := storage.New(input.Workspace)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var summary storage.Summary
	switch input.Action {
	case "pull":
		summary, err = backend.Pull(ctx, input.Root)
	case "push":
		summary, err = backend.Push(ctx, input.Root)
	default:
		return fmt.Errorf("unsupported action %q", input.Action)
	}
	if err != nil {
		return fmt.Errorf("%s workspace: %w", input.Action, err)
	}
	return json.NewEncoder(os.Stdout).Encode(response{OK: true, Summary: summary})
}
