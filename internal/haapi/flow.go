package haapi

import (
	"context"
	"encoding/json"
	"fmt"
)

// FlowResult represents a response from the config entries flow API.
type FlowResult struct {
	FlowID      string            `json:"flow_id"`
	Type        string            `json:"type"` // "form", "create_entry", "abort", "external", "menu"
	StepID      string            `json:"step_id"`
	Handler     string            `json:"handler"`
	Title       string            `json:"title"`
	Description string            `json:"description_placeholders,omitempty"`
	DataSchema  []SchemaField     `json:"-"` // parsed from raw data_schema
	Errors      map[string]string `json:"errors,omitempty"`
	Result      json.RawMessage   `json:"result,omitempty"`
}

// SchemaField describes one field in a flow step's data schema.
type SchemaField struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Type     string `json:"type,omitempty"` // "string", "integer", "boolean", "float", "select", etc.
	Default  any    `json:"default,omitempty"`
}

// flowRawResponse is the raw shape of the HA flow API response, used for parsing.
type flowRawResponse struct {
	FlowID     string            `json:"flow_id"`
	Type       string            `json:"type"`
	StepID     string            `json:"step_id"`
	Handler    string            `json:"handler"`
	Title      string            `json:"title"`
	DataSchema []json.RawMessage `json:"data_schema"`
	Errors     map[string]string `json:"errors"`
	Result     json.RawMessage   `json:"result"`
}

// parseFlowResult converts raw JSON into a FlowResult with parsed schema.
func parseFlowResult(data []byte) (*FlowResult, error) {
	var raw flowRawResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing flow response: %w", err)
	}

	result := &FlowResult{
		FlowID:  raw.FlowID,
		Type:    raw.Type,
		StepID:  raw.StepID,
		Handler: raw.Handler,
		Title:   raw.Title,
		Errors:  raw.Errors,
		Result:  raw.Result,
	}

	for _, fieldRaw := range raw.DataSchema {
		var field struct {
			Name     string `json:"name"`
			Required bool   `json:"required"`
			Type     string `json:"type"`
			Default  any    `json:"default"`
		}
		if err := json.Unmarshal(fieldRaw, &field); err == nil {
			result.DataSchema = append(result.DataSchema, SchemaField{
				Name:     field.Name,
				Required: field.Required,
				Type:     field.Type,
				Default:  field.Default,
			})
		}
	}

	return result, nil
}

// StartOptionsFlow starts an options flow for an existing config entry.
// POST /api/config/config_entries/options/flow with {"handler": entryID}
func (c *Client) StartOptionsFlow(ctx context.Context, entryID string) ([]byte, error) {
	body := map[string]string{"handler": entryID}
	return c.doPost(ctx, "/api/config/config_entries/options/flow", body)
}

// StartConfigFlow starts a new config flow for a domain/integration.
// POST /api/config/config_entries/flow with {"handler": domain}
func (c *Client) StartConfigFlow(ctx context.Context, domain string) ([]byte, error) {
	body := map[string]string{"handler": domain}
	return c.doPost(ctx, "/api/config/config_entries/flow", body)
}

// StepFlow submits data to advance a config/options flow.
// If options is true: POST /api/config/config_entries/options/flow/<flow_id>
// If options is false: POST /api/config/config_entries/flow/<flow_id>
func (c *Client) StepFlow(ctx context.Context, flowID string, options bool, data json.RawMessage) ([]byte, error) {
	if data == nil {
		data = json.RawMessage("{}")
	}
	path := "/api/config/config_entries/flow/" + flowID
	if options {
		path = "/api/config/config_entries/options/flow/" + flowID
	}
	return c.doPost(ctx, path, data)
}

// InspectFlow retrieves the current state of a flow.
// If options is true: GET /api/config/config_entries/options/flow/<flow_id>
// If options is false: GET /api/config/config_entries/flow/<flow_id>
func (c *Client) InspectFlow(ctx context.Context, flowID string, options bool) ([]byte, error) {
	path := "/api/config/config_entries/flow/" + flowID
	if options {
		path = "/api/config/config_entries/options/flow/" + flowID
	}
	return c.doGet(ctx, path)
}

// ParseFlowResult parses raw flow API response bytes into a structured FlowResult.
func ParseFlowResult(data []byte) (*FlowResult, error) {
	return parseFlowResult(data)
}
