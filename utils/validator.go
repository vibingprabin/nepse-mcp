package utils

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var symbolRegex = regexp.MustCompile(`^[A-Z0-9]+$`)

// ValidateSymbol checks if a security symbol is valid (uppercase alphanumeric)
func ValidateSymbol(symbol string) error {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	if s == "" {
		return fmt.Errorf("symbol cannot be empty")
	}
	if len(s) > 20 {
		return fmt.Errorf("symbol too long: %s", symbol)
	}
	if !symbolRegex.MatchString(s) {
		return fmt.Errorf("invalid symbol format: %s (must be alphanumeric)", symbol)
	}
	return nil
}

// ValidateDate checks if a date string matches YYYY-MM-DD format
func ValidateDate(dateStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, fmt.Errorf("date cannot be empty")
	}
	
	layout := "2006-01-02"
	parsedDate, err := time.Parse(layout, dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format: %s (expected YYYY-MM-DD)", dateStr)
	}
	
	// Date shouldn't be in the future
	if parsedDate.After(time.Now().Add(24 * time.Hour)) {
		return time.Time{}, fmt.Errorf("date cannot be in the future: %s", dateStr)
	}
	
	// Date shouldn't be too old (reasonable limit for stock data)
	minDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if parsedDate.Before(minDate) {
		return time.Time{}, fmt.Errorf("date too old: %s (minimum: 2000-01-01)", dateStr)
	}

	return parsedDate, nil
}

// ValidateDateRange checks if start date is before end date and range doesn't exceed max days
func ValidateDateRange(startStr, endStr string, maxDays int) (time.Time, time.Time, error) {
	start, err := ValidateDate(startStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("start date: %w", err)
	}
	
	end, err := ValidateDate(endStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("end date: %w", err)
	}
	
	if start.After(end) {
		return time.Time{}, time.Time{}, fmt.Errorf("start date (%s) cannot be after end date (%s)", startStr, endStr)
	}
	
	if maxDays > 0 {
		daysDiff := int(end.Sub(start).Hours() / 24)
		if daysDiff > maxDays {
			return time.Time{}, time.Time{}, fmt.Errorf("date range (%d days) exceeds maximum of %d days", daysDiff, maxDays)
		}
	}
	
	return start, end, nil
}
