package main

import (
	"encoding/base64"
	"fmt"
)

type Job struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	PrinterIdentifier string `json:"printer_identifier"`
	JobType           string `json:"job_type"`
	PayloadB64        string `json:"payload"`
	LeaseUUID         string `json:"lease_uuid"`
}

func (j Job) DecodePayload() ([]byte, error) {
	if j.PayloadB64 == "" {
		return nil, fmt.Errorf("missing payload")
	}
	b, err := base64.StdEncoding.DecodeString(j.PayloadB64)
	if err != nil {
		return nil, err
	}
	return b, nil
}
