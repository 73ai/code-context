package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Constants for various utility functions
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
	MinPasswordLength = 8
	EmailPattern    = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
)

var (
	emailRegex = regexp.MustCompile(EmailPattern)
)

// StringUtils provides string manipulation utilities
type StringUtils struct{}

// IsEmpty checks if a string is empty or contains only whitespace
func (StringUtils) IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// Capitalize capitalizes the first letter of a string
func (StringUtils) Capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// TruncateString truncates a string to a specified length
func (StringUtils) TruncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}

// GenerateSlug creates a URL-friendly slug from a string
func (StringUtils) GenerateSlug(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, "-")
	s = regexp.MustCompile(`-+`).ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// ValidationUtils provides data validation utilities
type ValidationUtils struct{}

// IsValidEmail validates an email address format
func (ValidationUtils) IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

// IsStrongPassword checks if a password meets strength requirements
func (ValidationUtils) IsStrongPassword(password string) bool {
	if len(password) < MinPasswordLength {
		return false
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasDigit && hasSpecial
}

// ValidatePageParams validates pagination parameters
func (ValidationUtils) ValidatePageParams(page, pageSize int) (int, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return page, pageSize, nil
}

// CryptoUtils provides cryptographic utilities
type CryptoUtils struct{}

// GenerateRandomString generates a random string of specified length
func (CryptoUtils) GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}

// HashPassword creates a SHA256 hash of a password (for demo purposes)
func (CryptoUtils) HashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// GenerateToken generates a secure random token
func (CryptoUtils) GenerateToken() (string, error) {
	return cu.GenerateRandomString(32)
}

// TimeUtils provides time-related utilities
type TimeUtils struct{}

// FormatDuration formats a duration in human-readable format
func (TimeUtils) FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	return fmt.Sprintf("%.1fd", d.Hours()/24)
}

// ParseDateString parses various date string formats
func (TimeUtils) ParseDateString(dateStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05Z07:00",
		"01/02/2006",
		"01-02-2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// IsBusinessDay checks if a given date is a business day (Monday-Friday)
func (TimeUtils) IsBusinessDay(t time.Time) bool {
	weekday := t.Weekday()
	return weekday >= time.Monday && weekday <= time.Friday
}

// MathUtils provides mathematical utilities
type MathUtils struct{}

// Round rounds a float64 to a specified number of decimal places
func (MathUtils) Round(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return math.Round(num*output) / output
}

// Percentage calculates the percentage of part relative to total
func (MathUtils) Percentage(part, total float64) float64 {
	if total == 0 {
		return 0
	}
	return (part / total) * 100
}

// Clamp constrains a value between min and max
func (MathUtils) Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ConversionUtils provides type conversion utilities
type ConversionUtils struct{}

// StringToInt safely converts a string to an integer
func (ConversionUtils) StringToInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// StringToFloat safely converts a string to a float64
func (ConversionUtils) StringToFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// IntToString converts an integer to a string
func (ConversionUtils) IntToString(i int) string {
	return strconv.Itoa(i)
}

// BoolToString converts a boolean to a string
func (ConversionUtils) BoolToString(b bool) string {
	return strconv.FormatBool(b)
}

// Global utility instances
var (
	su = StringUtils{}
	vu = ValidationUtils{}
	cu = CryptoUtils{}
	tu = TimeUtils{}
	mu = MathUtils{}
	cu2 = ConversionUtils{}
)

// Helper functions that use the utility instances

// ProcessUserInput processes and validates user input
func ProcessUserInput(input string) (string, error) {
	if su.IsEmpty(input) {
		return "", fmt.Errorf("input cannot be empty")
	}

	processed := su.TruncateString(strings.TrimSpace(input), 255)
	return processed, nil
}

// ValidateAndFormatEmail validates and formats an email address
func ValidateAndFormatEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !vu.IsValidEmail(email) {
		return "", fmt.Errorf("invalid email format")
	}
	return email, nil
}

// CalculateOffset calculates database offset for pagination
func CalculateOffset(page, pageSize int) int {
	return (page - 1) * pageSize
}