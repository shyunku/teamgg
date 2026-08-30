package main

import "strings"

func redactValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "token") || strings.Contains(lower, "secret") ||
				strings.Contains(lower, "password") || strings.Contains(lower, "cookie") ||
				strings.Contains(lower, "authorization") {
				result[key] = "[REDACTED]"
				continue
			}
			result[key] = redactValue(nested)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(typed))
		for index, nested := range typed {
			result[index] = redactValue(nested)
		}
		return result
	case string:
		if len(typed) > 500 {
			return typed[:500]
		}
		return typed
	default:
		return value
	}
}
