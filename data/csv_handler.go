package data

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ScripData represents a single stock entry from Scrips.csv
type ScripData struct {
	Symbol           string
	Company          string
	LTP              float64
	ShareOutstanding float64
	FloatedShares    float64
	MarketCap        float64
	FloatedMarketCap float64
	ImpactInNEPSE    string
	AvgVolume50D     float64
}

// LoadScripsCSV loads the Scrips.csv file
func LoadScripsCSV(filepath string) ([]ScripData, map[string]*ScripData, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, nil, fmt.Errorf("opening scrips file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	if _, err := reader.Read(); err != nil {
		return nil, nil, fmt.Errorf("reading header: %w", err)
	}

	var scrips []ScripData
	scripMap := make(map[string]*ScripData)
	for {
		record, err := reader.Read()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, nil, fmt.Errorf("reading CSV: %w", err)
		}

		if len(record) < 9 {
			continue
		}

		scrip := ScripData{
			Symbol:        strings.TrimSpace(record[0]),
			Company:       strings.TrimSpace(record[1]),
			ImpactInNEPSE: strings.TrimSpace(record[7]),
		}

		scrip.LTP = parseFloat(record[2])
		scrip.ShareOutstanding = parseFloat(record[3])
		scrip.FloatedShares = parseFloat(record[4])
		scrip.MarketCap = parseFloat(record[5])
		scrip.FloatedMarketCap = parseFloat(record[6])
		scrip.AvgVolume50D = parseFloat(record[8])

		scrips = append(scrips, scrip)
		scripMap[strings.ToUpper(scrip.Symbol)] = &scrips[len(scrips)-1]
	}

	return scrips, scripMap, nil
}

// GetScripBySymbol returns a single scrip by symbol
func GetScripBySymbol(scripMap map[string]*ScripData, symbol string) *ScripData {
	return scripMap[strings.ToUpper(strings.TrimSpace(symbol))]
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	val, _ := strconv.ParseFloat(s, 64)
	return val
}