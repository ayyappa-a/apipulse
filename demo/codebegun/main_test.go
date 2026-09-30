package main

import "testing"

func TestNormalizeEndpoint(t *testing.T) {
    tests := map[string]string{
        "http://api:8080":    "api:8080",
        "https://api:8080":   "api:8080",
        "api:8080":          "api:8080",
        "http://localhost":  "localhost",
        "https://example.com": "example.com",
    }
    for input, expected := range tests {
        got := normalizeEndpoint(input)
        if got != expected {
            t.Fatalf("normalizeEndpoint(%s) = %s; want %s", input, got, expected)
        }
    }
}
