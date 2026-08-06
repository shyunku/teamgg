package riot

import (
	"fmt"
	"net/url"
)

func Encode(query string) string {
	return url.PathEscape(query)
}

func CreateUrl(region string, path string) string {
	return fmt.Sprintf("https://%s.api.riotgames.com%s?api_key=%s", region, path, apiKey)
}

func CreateUrlWithQuery(region string, path string, queries map[string]interface{}) string {
	query := ""
	for key, value := range queries {
		valueStr := fmt.Sprintf("%v", value)
		query += fmt.Sprintf("&%s=%v", Encode(key), Encode(valueStr))
	}
	return fmt.Sprintf("https://%s.api.riotgames.com%s?api_key=%s%s", region, path, apiKey, query)
}
