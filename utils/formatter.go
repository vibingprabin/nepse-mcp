package utils

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	api "github.com/voidarchive/go-nepse"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var enPrinter = message.NewPrinter(language.English)

// FormatCurrency formats a float64 as a currency string (e.g., "Rs. 1,234.56").
func FormatCurrency(amount float64) string {
	return enPrinter.Sprintf("Rs. %.2f", amount)
}

// FormatPercentage formats a float64 as a percentage string (e.g., "+12.34%").
func FormatPercentage(value float64) string {
	sign := ""
	if value > 0 {
		sign = "+"
	}
	return fmt.Sprintf("%s%.2f%%", sign, value)
}

// FormatVolume formats a large number with commas (e.g., "1,234,567").
func FormatVolume(volume int64) string {
	return enPrinter.Sprintf("%d", volume)
}

// FormatTrend returns a text indicator based on the value (positive/negative/neutral).
func FormatTrend(change float64) string {
	if change > 0 {
		return "UP"
	} else if change < 0 {
		return "DOWN"
	}
	return "FLAT"
}

// FormatNullableFloat handles potentially nil or zero float values for display
func FormatNullableFloat(val *float64) string {
	if val == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f", *val)
}

// FormatNumber formats a float64 with commas (e.g., "1,234.56").
func FormatNumber(num float64) string {
	return enPrinter.Sprintf("%.2f", num)
}

// HumanizeNumber formats large numbers into readable suffix format (K, M, B)
func HumanizeNumber(num float64) string {
	if math.Abs(num) < 1000 {
		return fmt.Sprintf("%.2f", num)
	}

	suffixes := []string{"", "K", "M", "B", "T"}
	exp := int(math.Log10(math.Abs(num)) / 3)
	if exp >= len(suffixes) {
		exp = len(suffixes) - 1
	}

	value := num / math.Pow(1000, float64(exp))
	return fmt.Sprintf("%.2f%s", value, suffixes[exp])
}

// FastParseFloat efficiently parses a float string, removing commas if present
func FastParseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if i := strings.IndexByte(s, ','); i >= 0 {
		b := make([]byte, 0, len(s))
		for i := 0; i < len(s); i++ {
			if s[i] != ',' {
				b = append(b, s[i])
			}
		}
		s = string(b)
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// ReversePriceHistory reverses a slice of PriceHistory in place
func ReversePriceHistory(s []api.PriceHistory) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
