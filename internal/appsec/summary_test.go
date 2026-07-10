package appsec

import (
	"testing"

	datadogV2 "github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

func TestAggregateBlockedRules(t *testing.T) {
	spans := []datadogV2.Span{
		{
			Attributes: &datadogV2.SpansAttributes{
				Custom: map[string]interface{}{
					"appsec": map[string]interface{}{
						"type": []interface{}{"lfi"},
						"triggers": []interface{}{
							map[string]interface{}{
								"rule": map[string]interface{}{
									"id":   "crs-930-100",
									"name": "Obfuscated Path Traversal Attack (/../)",
									"tags": map[string]interface{}{
										"type":     "lfi",
										"category": "attack_attempt",
										"module":   "waf",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Attributes: &datadogV2.SpansAttributes{
				Custom: map[string]interface{}{
					"appsec": map[string]interface{}{
						"type": []interface{}{"block_ip"},
						"triggers": []interface{}{
							map[string]interface{}{
								"rule": map[string]interface{}{
									"id":   "blk-001-001",
									"name": "Block IP Addresses",
									"tags": map[string]interface{}{
										"type":     "block_ip",
										"category": "security_response",
										"module":   "network-acl",
									},
								},
							},
							map[string]interface{}{
								"rule": map[string]interface{}{
									"id":   "crs-930-100",
									"name": "Obfuscated Path Traversal Attack (/../)",
									"tags": map[string]interface{}{
										"type": "lfi",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	got := AggregateBlockedRules(spans)
	if got.SpansScanned != 2 {
		t.Fatalf("expected 2 spans scanned, got %d", got.SpansScanned)
	}
	if len(got.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(got.Rules))
	}
	if got.Rules[0].ID != "crs-930-100" || got.Rules[0].Count != 2 {
		t.Fatalf("expected crs-930-100 count 2 first, got %#v", got.Rules[0])
	}
	if len(got.Types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(got.Types))
	}
}
