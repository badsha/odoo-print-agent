package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type APIClient struct {
	baseURL *url.URL
	apiKey  string
	http    *http.Client
}

func NewAPIClient(odooURL, apiKey string) *APIClient {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(odooURL), "/"))
	if err != nil {
		panic(err)
	}
	if strings.EqualFold(u.Hostname(), "localhost") {
		port := u.Port()
		if port == "" {
			u.Host = "127.0.0.1"
		} else {
			u.Host = net.JoinHostPort("127.0.0.1", port)
		}
	}
	return &APIClient{
		baseURL: u,
		apiKey:  strings.TrimSpace(apiKey),
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *APIClient) SyncPrinters(ctx context.Context, printers []PrinterConfig) error {
	type printer struct {
		AgentIdentifier string `json:"agent_identifier"`
		Name            string `json:"name"`
		PrinterType     string `json:"printer_type"`
		Code            string `json:"code"`
	}
	payload := struct {
		Printers []printer `json:"printers"`
	}{Printers: make([]printer, 0, len(printers))}

	for _, p := range printers {
		payload.Printers = append(payload.Printers, printer{
			AgentIdentifier: p.AgentIdentifier,
			Name:            p.Name,
			PrinterType:     p.PrinterType,
			Code:            p.Code,
		})
	}
	var resp struct {
		Status    string `json:"status"`
		Message   string `json:"message"`
		PrinterID []int  `json:"printer_ids"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/print/printers/sync", nil, payload, &resp); err != nil {
		return err
	}
	if strings.ToLower(strings.TrimSpace(resp.Status)) != "success" {
		msg := strings.TrimSpace(resp.Message)
		if msg == "" {
			msg = "unknown error"
		}
		return fmt.Errorf("odoo error: %s", msg)
	}
	return nil
}

func (c *APIClient) GetJobs(ctx context.Context, leaseSeconds int, limit int) ([]Job, error) {
	q := make(url.Values)
	if leaseSeconds > 0 {
		q.Set("lease_seconds", strconv.Itoa(leaseSeconds))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var resp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Jobs    []Job  `json:"jobs"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/print/jobs", q, nil, &resp); err != nil {
		return nil, err
	}
	if strings.ToLower(resp.Status) != "success" {
		return nil, fmt.Errorf("odoo error: %s", strings.TrimSpace(resp.Message))
	}
	return resp.Jobs, nil
}

func (c *APIClient) AckJob(ctx context.Context, jobID int64, leaseUUID string) error {
	body := map[string]string{"lease_uuid": leaseUUID}
	var resp map[string]any
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/print/job/%d/ack", jobID), nil, body, &resp)
}

func (c *APIClient) DoneJob(ctx context.Context, jobID int64, leaseUUID string) error {
	body := map[string]string{"lease_uuid": leaseUUID}
	var resp map[string]any
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/print/job/%d/done", jobID), nil, body, &resp)
}

func (c *APIClient) FailJob(ctx context.Context, jobID int64, leaseUUID string, errorMessage string) error {
	body := map[string]string{"lease_uuid": leaseUUID, "error_message": errorMessage}
	var resp map[string]any
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/print/job/%d/fail", jobID), nil, body, &resp)
}

func (c *APIClient) doJSON(ctx context.Context, method string, endpoint string, query url.Values, reqBody any, out any) error {
	u := *c.baseURL
	u.Path = path.Join(u.Path, endpoint)
	u.RawQuery = query.Encode()

	var body io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(res.Body, 10<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("http %d: %s", res.StatusCode, strings.TrimSpace(string(respBytes)))
	}
	if out == nil {
		return nil
	}
	if len(respBytes) == 0 {
		return nil
	}
	return json.Unmarshal(respBytes, out)
}
