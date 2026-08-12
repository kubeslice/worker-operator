package workerslicegwrecycler

import (
	"testing"
)

func TestGetNewDeploymentName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test-slice-gw-0", "test-slice-gw-1"},
		{"test-slice-gw-1", "test-slice-gw-0"},
		{"another-gw-0", "another-gw-1"},
		{"another-gw-1", "another-gw-0"},
	}

	for _, test := range tests {
		result := getNewDeploymentName(test.input)
		if result != test.expected {
			t.Errorf("For input %s, expected %s but got %s", test.input, test.expected, result)
		}
	}
}

func TestGetRequestString(t *testing.T) {
	tests := []struct {
		input    Request
		expected string
	}{
		{REQ_none, "none"},
		{REQ_create_new_deployment, "create_new_deployment"},
		{REQ_update_routing_table, "update_routing_table"},
		{REQ_delete_old_gw_deployment, "delete_old_gw_deployment"},
		{REQ_invalid, ""},
	}

	for _, test := range tests {
		result := getRequestString(test.input)
		if result != test.expected {
			t.Errorf("For input %v, expected %s but got %s", test.input, test.expected, result)
		}
	}
}

func TestGetRequestIndex(t *testing.T) {
	tests := []struct {
		input    string
		expected Request
	}{
		{"none", REQ_none},
		{"create_new_deployment", REQ_create_new_deployment},
		{"update_routing_table", REQ_update_routing_table},
		{"delete_old_gw_deployment", REQ_delete_old_gw_deployment},
		{"unknown", REQ_invalid},
	}

	for _, test := range tests {
		result := getRequestIndex(test.input)
		if result != test.expected {
			t.Errorf("For input %s, expected %v but got %v", test.input, test.expected, result)
		}
	}
}

func TestGetResponseString(t *testing.T) {
	tests := []struct {
		input    Response
		expected string
	}{
		{RESP_none, "none"},
		{RESP_new_deployment_created, "new_deployment_created"},
		{RESP_routing_table_updated, "routing_table_updated"},
		{RESP_old_deployment_deleted, "old_gw_deployment_deleted"},
		{RESP_invalid, ""},
	}

	for _, test := range tests {
		result := getResponseString(test.input)
		if result != test.expected {
			t.Errorf("For input %v, expected %s but got %s", test.input, test.expected, result)
		}
	}
}

func TestGetResponseIndex(t *testing.T) {
	tests := []struct {
		input    string
		expected Response
	}{
		{"none", RESP_none},
		{"new_deployment_created", RESP_new_deployment_created},
		{"routing_table_updated", RESP_routing_table_updated},
		{"old_gw_deployment_deleted", RESP_old_deployment_deleted},
		{"unknown", RESP_invalid},
	}

	for _, test := range tests {
		result := getResponseIndex(test.input)
		if result != test.expected {
			t.Errorf("For input %s, expected %v but got %v", test.input, test.expected, result)
		}
	}
}

func TestRequestResponseRoundTrip(t *testing.T) {
	// Test that converting Request to string and back gives the same result
	requests := []Request{REQ_none, REQ_create_new_deployment, REQ_update_routing_table, REQ_delete_old_gw_deployment}
	for _, req := range requests {
		str := getRequestString(req)
		result := getRequestIndex(str)
		if result != req {
			t.Errorf("Request round trip failed: %v -> %s -> %v", req, str, result)
		}
	}

	// Test that converting Response to string and back gives the same result
	responses := []Response{RESP_none, RESP_new_deployment_created, RESP_routing_table_updated, RESP_old_deployment_deleted}
	for _, resp := range responses {
		str := getResponseString(resp)
		result := getResponseIndex(str)
		if result != resp {
			t.Errorf("Response round trip failed: %v -> %s -> %v", resp, str, result)
		}
	}
}
