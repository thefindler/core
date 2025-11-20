package logger

import (
	"encoding/json"
	"regexp"
	"strings"
)

// PII masking regular expressions compiled once for better performance
var (
	// Aadhaar: 12 digits (can have hyphens or spaces)
	aadhaarRegex = regexp.MustCompile(`\b\d{4}[- ]?\d{4}[- ]?\d{4}\b`)
	
	// PAN: 5 letters + 4 digits + 1 letter (case insensitive)
	panRegex = regexp.MustCompile(`(?i)\b[A-Z]{5}[0-9]{4}[A-Z]{1}\b`)
	
	// Phone: Indian phone numbers only (not channel IDs or other numeric data)
	// Matches +91XXXXXXXXXX or 10-digit numbers that start with 6,7,8,9 (valid Indian mobile prefixes)
	phoneRegex = regexp.MustCompile(`\b(\+91[- ]?[6-9]\d{9}|[6-9]\d{9})\b`)
	
	// Email: standard email pattern
	emailRegex = regexp.MustCompile(`([a-zA-Z0-9._%+-])[^@]*(@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)
	
	// Credit Card: 13-19 digits with optional spaces/hyphens
	creditCardRegex = regexp.MustCompile(`\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{3,4}\b`)
	
	// Additional Indian patterns
	// Voter ID: 3 letters followed by 7 digits
	voterIdRegex = regexp.MustCompile(`(?i)\b[A-Z]{3}[0-9]{7}\b`)
	
	// Driving License: varies by state, but generally alphanumeric
	drivingLicenseRegex = regexp.MustCompile(`(?i)\b[A-Z]{2}[-]?[0-9]{2}[-]?[0-9]{11}\b`)
)

// maskPIIString masks PII data in a string for ISO 27001 compliance
// This function identifies and masks various types of sensitive data including:
// - Aadhaar numbers, PAN cards, phone numbers, emails, credit cards, etc.
// Note: Phone numbers show last 4 digits for debugging purposes (standard practice for compliance)
func maskPIIString(input string) string {
	if input == "" {
		return input
	}
	
	result := input
	
	// Mask Aadhaar numbers: show only last 4 digits
	result = aadhaarRegex.ReplaceAllStringFunc(result, func(s string) string {
		// Remove spaces/hyphens for processing
		cleaned := strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), "-", "")
		if len(cleaned) >= 4 {
			return "XXXX-XXXX-" + cleaned[len(cleaned)-4:]
		}
		return "XXXX-XXXX-XXXX"
	})
	
	// Mask PAN numbers: show first 5 and last 1 character
	result = panRegex.ReplaceAllStringFunc(result, func(s string) string {
		if len(s) >= 6 {
			return s[:5] + "XXXX" + s[len(s)-1:]
		}
		return "XXXXXXXXXXXXX"
	})
	
	// Mask phone numbers: show only last 4 digits for debugging purposes
	result = phoneRegex.ReplaceAllStringFunc(result, func(s string) string {
		// Extract digits only
		digits := regexp.MustCompile(`\d`).FindAllString(s, -1)
		if len(digits) >= 4 {
			lastFour := strings.Join(digits[len(digits)-4:], "")
			// Preserve country code format if present
			if strings.HasPrefix(s, "+91") {
				return "+91XXXXXX" + lastFour
			}
			return "XXXXXX" + lastFour
		}
		return "XXXXXXXXXX"
	})
	
	// Mask email addresses: show first character and domain
	result = emailRegex.ReplaceAllString(result, "$1***$2")
	
	// Mask credit card numbers: show only last 4 digits
	result = creditCardRegex.ReplaceAllStringFunc(result, func(s string) string {
		// Extract digits only
		digits := regexp.MustCompile(`\d`).FindAllString(s, -1)
		if len(digits) >= 4 {
			lastFour := strings.Join(digits[len(digits)-4:], "")
			return "XXXX-XXXX-XXXX-" + lastFour
		}
		return "XXXX-XXXX-XXXX-XXXX"
	})
	
	// Mask Voter ID
	result = voterIdRegex.ReplaceAllStringFunc(result, func(s string) string {
		if len(s) >= 4 {
			return s[:3] + "XXXX" + s[len(s)-3:]
		}
		return "XXXXXXXXXX"
	})
	
	// Mask Driving License
	result = drivingLicenseRegex.ReplaceAllStringFunc(result, func(s string) string {
		if len(s) >= 6 {
			return s[:4] + "XXXXXXXXX" + s[len(s)-2:]
		}
		return "XXXXXXXXXXXXXXX"
	})
	
	return result
}

// maskPIIData recursively masks PII in data maps
// This ensures that any nested maps or slices also get their PII data masked
func maskPIIData(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}
	
	masked := make(map[string]interface{})
	for key, value := range data {
		masked[key] = maskPIIValue(value)
	}
	return masked
}

// maskPIIValue masks PII in various data types
func maskPIIValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return maskPIIString(v)
	case map[string]interface{}:
		return maskPIIData(v)
	case []interface{}:
		masked := make([]interface{}, len(v))
		for i, item := range v {
			masked[i] = maskPIIValue(item)
		}
		return masked
	case []string:
		masked := make([]string, len(v))
		for i, item := range v {
			masked[i] = maskPIIString(item)
		}
		return masked
	default:
		// Handle other types that might contain nested data
		// This includes struct types that were serialized to interface{}
		return maskGenericValue(value)
	}
}

// maskGenericValue handles complex types by converting to map and masking
func maskGenericValue(value interface{}) interface{} {
	// Try to convert to JSON and back to map[string]interface{} for deep masking
	if jsonBytes, err := json.Marshal(value); err == nil {
		var mapValue map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &mapValue); err == nil {
			return maskPIIData(mapValue)
		}
		// If it's an array/slice
		var arrayValue []interface{}
		if err := json.Unmarshal(jsonBytes, &arrayValue); err == nil {
			masked := make([]interface{}, len(arrayValue))
			for i, item := range arrayValue {
				masked[i] = maskPIIValue(item)
			}
			return masked
		}
		// If it's a simple value, try to mask as string
		var stringValue string
		if err := json.Unmarshal(jsonBytes, &stringValue); err == nil {
			return maskPIIString(stringValue)
		}
	}
	// Return as-is if we can't process it
	return value
}

