package appsec

import (
	"sort"

	datadogV2 "github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

type RuleSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"`
	Category string `json:"category,omitempty"`
	Module   string `json:"module,omitempty"`
	Count    int    `json:"count"`
}

type TypeSummary struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type BlockedRulesSummary struct {
	SpansScanned int           `json:"spans_scanned"`
	Rules        []RuleSummary `json:"rules"`
	Types        []TypeSummary `json:"types"`
}

func AggregateBlockedRules(spans []datadogV2.Span) BlockedRulesSummary {
	ruleCounts := map[string]*RuleSummary{}
	typeCounts := map[string]int{}

	for _, span := range spans {
		attrs, ok := span.GetAttributesOk()
		if !ok || attrs == nil {
			continue
		}
		appsec, ok := attrs.Custom["appsec"].(map[string]interface{})
		if !ok {
			continue
		}

		for _, typeName := range stringList(appsec["type"]) {
			typeCounts[typeName]++
		}

		triggers, ok := appsec["triggers"].([]interface{})
		if !ok {
			continue
		}
		for _, trigger := range triggers {
			triggerMap, ok := trigger.(map[string]interface{})
			if !ok {
				continue
			}
			ruleMap, ok := triggerMap["rule"].(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := ruleMap["id"].(string)
			if id == "" {
				continue
			}
			entry, exists := ruleCounts[id]
			if !exists {
				entry = &RuleSummary{ID: id}
				if name, ok := ruleMap["name"].(string); ok {
					entry.Name = name
				}
				if tags, ok := ruleMap["tags"].(map[string]interface{}); ok {
					if typeName, ok := tags["type"].(string); ok {
						entry.Type = typeName
					}
					if category, ok := tags["category"].(string); ok {
						entry.Category = category
					}
					if module, ok := tags["module"].(string); ok {
						entry.Module = module
					}
				}
				ruleCounts[id] = entry
			}
			entry.Count++
		}
	}

	rules := make([]RuleSummary, 0, len(ruleCounts))
	for _, rule := range ruleCounts {
		rules = append(rules, *rule)
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Count == rules[j].Count {
			return rules[i].ID < rules[j].ID
		}
		return rules[i].Count > rules[j].Count
	})

	types := make([]TypeSummary, 0, len(typeCounts))
	for typeName, count := range typeCounts {
		types = append(types, TypeSummary{Type: typeName, Count: count})
	}
	sort.Slice(types, func(i, j int) bool {
		if types[i].Count == types[j].Count {
			return types[i].Type < types[j].Type
		}
		return types[i].Count > types[j].Count
	})

	return BlockedRulesSummary{
		SpansScanned: len(spans),
		Rules:        rules,
		Types:        types,
	}
}

func stringList(value interface{}) []string {
	switch typed := value.(type) {
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if typed != "" {
			return []string{typed}
		}
	}
	return nil
}
