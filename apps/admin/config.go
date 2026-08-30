package main

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	host             string
	port             string
	teamggAPIBaseURL string
	replayAPIBaseURL string
	internalSecret   string
	allowedOrigins   map[string]struct{}
	requestTimeout   time.Duration
	shutdownTimeout  time.Duration
}

func loadConfig() (config, error) {
	result := config{
		host:             envOrDefault("HOST", "0.0.0.0"),
		port:             envOrDefault("PORT", "7730"),
		teamggAPIBaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("TEAMGG_API_BASE_URL")), "/"),
		replayAPIBaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("REPLAY_ANALYZER_BASE_URL")), "/"),
		internalSecret:   strings.TrimSpace(os.Getenv("ADMIN_INTERNAL_SECRET")),
		allowedOrigins:   parseOrigins(envOrDefault("ADMIN_ALLOWED_ORIGINS", "http://localhost:8080,https://teamgg.kr,https://www.teamgg.kr")),
		requestTimeout:   durationOrDefault("ADMIN_REQUEST_TIMEOUT", 5*time.Second),
		shutdownTimeout:  durationOrDefault("ADMIN_SHUTDOWN_TIMEOUT", 10*time.Second),
	}
	if result.teamggAPIBaseURL == "" || result.replayAPIBaseURL == "" || result.internalSecret == "" {
		return config{}, errors.New("TEAMGG_API_BASE_URL, REPLAY_ANALYZER_BASE_URL, and ADMIN_INTERNAL_SECRET are required")
	}
	if _, err := strconv.Atoi(result.port); err != nil {
		return config{}, errors.New("PORT must be numeric")
	}
	return result, nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationOrDefault(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 || parsed > time.Minute {
		return fallback
	}
	return parsed
}

func parseOrigins(value string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, origin := range strings.Split(value, ",") {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin != "" {
			result[origin] = struct{}{}
		}
	}
	return result
}
