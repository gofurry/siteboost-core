package privilege

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func readHelperRequest(path string) (HelperRequest, error) {
	var req HelperRequest
	data, err := os.ReadFile(path)
	if err != nil {
		return req, fmt.Errorf("read helper request: %w", err)
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return req, fmt.Errorf("parse helper request: %w", err)
	}
	return req, nil
}

func writeHelperRequest(path string, req HelperRequest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create helper request directory: %w", err)
	}
	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return fmt.Errorf("encode helper request: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write helper request: %w", err)
	}
	return nil
}

func readHelperResponse(path string) (HelperResponse, error) {
	var resp HelperResponse
	data, err := os.ReadFile(path)
	if err != nil {
		return resp, err
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, fmt.Errorf("parse helper response: %w", err)
	}
	return resp, nil
}

func writeHelperResponse(path string, resp HelperResponse) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create helper response directory: %w", err)
	}
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return fmt.Errorf("encode helper response: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write helper response: %w", err)
	}
	return nil
}
