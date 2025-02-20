package main

import (
	"strings"
)

func SanitizeConfigOutput(content string) (error, string) {
	lines := strings.Split(content, "\n")
	// Check if the first line contains '#'
	if len(lines) > 0 && strings.Contains(lines[0], "show") {
		// Remove the first line
		lines = lines[1:]
	}

	// Check if the second line contains '#'
	if len(lines) > 0 && strings.Contains(lines[0], "Building configuration") {
		// Remove the second line
		lines = append(lines[:0], lines[1:]...)
	}

	// Check if the last line contains '#'
	if len(lines) > 0 && strings.Contains(lines[len(lines)-2], "#") {
		// Remove the last line
		lines = lines[:len(lines)-2]
	}

	// Join the sanitized lines back into a single string
	sanitizedOutput := strings.Join(lines, "\n")
	return nil, sanitizedOutput
}
