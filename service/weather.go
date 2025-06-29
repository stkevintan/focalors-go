package service

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"focalors-go/config"
	"focalors-go/slogger"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"resty.dev/v3"
)

//go:embed city-code.json
var cityCodeJSON embed.FS

// District represents a district/county under a city
type District struct {
	Name   string `json:"name"`
	Adcode string `json:"adcode"`
}

// City represents a city with its districts
type City struct {
	Name      string     `json:"name"`
	Adcode    string     `json:"adcode"`
	Districts []District `json:"districts,omitempty"`
}

// Province represents a province with its cities
type Province struct {
	Name   string `json:"name"`
	Adcode string `json:"adcode"`
	Cities []City `json:"cities,omitempty"`
}

// ChinaData represents the complete hierarchical structure
type ChinaData struct {
	Provinces []Province `json:"provinces"`
}

type CityInfo struct {
	Name     string
	Adcode   string
	CityCode string
}

type WeatherService struct {
	client    *resty.Client
	cfg       *config.WeatherConfig
	chinaData *ChinaData
	adcodeMap map[string]string // name -> adcode mapping for quick lookup
	initOnce  sync.Once
}

var logger = slogger.New("weather")

func NewWeatherService(cfg *config.WeatherConfig) *WeatherService {
	ws := &WeatherService{
		client:    resty.New().SetRetryCount(3).SetRetryWaitTime(1),
		cfg:       cfg,
		adcodeMap: make(map[string]string),
	}
	return ws
}

type WeatherLive struct {
	Province      string `json:"province"`
	City          string `json:"city"`
	Adcode        string `json:"adcode"`
	Weather       string `json:"weather"`
	Temperature   string `json:"temperature"`
	WindDirection string `json:"winddirection"`
	WindPower     string `json:"windpower"`
	Humidity      string `json:"humidity"`
	ReportTime    string `json:"reporttime"`
}

type WeatherData struct {
	Status   string        `json:"status"` // 1: success, 0: failed
	Count    string        `json:"count"`
	Info     string        `json:"info"`
	InfoCode string        `json:"infocode"` // 返回状态说明,10000代表正确
	Lives    []WeatherLive `json:"lives"`
}

func (w *WeatherService) initCityData() {
	w.initOnce.Do(func() {
		file, err := cityCodeJSON.Open("city-code.json")
		if err != nil {
			logger.Error("Failed to open city-code.json", slog.String("error", err.Error()))
			return
		}
		defer file.Close()

		var chinaData ChinaData
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&chinaData); err != nil {
			logger.Error("Failed to decode city-code.json", slog.String("error", err.Error()))
			return
		}

		w.chinaData = &chinaData
		w.buildAdcodeMap()
	})
}

func (w *WeatherService) buildAdcodeMap() {
	// Build a comprehensive mapping for quick lookup
	for _, province := range w.chinaData.Provinces {
		// Add province mappings (with and without suffix)
		w.adcodeMap[province.Name] = province.Adcode
		provinceName := w.normalizeProvinceCityName(province.Name)
		if provinceName != province.Name {
			w.adcodeMap[provinceName] = province.Adcode
		}

		for _, city := range province.Cities {
			// Add city mappings (with and without suffix)
			w.adcodeMap[city.Name] = city.Adcode
			cityName := w.normalizeProvinceCityName(city.Name)
			if cityName != city.Name {
				w.adcodeMap[cityName] = city.Adcode
			}

			// Add hierarchical mappings for cities
			w.adcodeMap[provinceName+cityName] = city.Adcode
			w.adcodeMap[provinceName+city.Name] = city.Adcode
			w.adcodeMap[province.Name+cityName] = city.Adcode

			for _, district := range city.Districts {
				// Add district mappings (districts keep their full names)
				w.adcodeMap[district.Name] = district.Adcode

				// Add hierarchical mappings for districts
				w.adcodeMap[cityName+district.Name] = district.Adcode
				w.adcodeMap[city.Name+district.Name] = district.Adcode
				w.adcodeMap[provinceName+cityName+district.Name] = district.Adcode
				w.adcodeMap[provinceName+city.Name+district.Name] = district.Adcode
				w.adcodeMap[province.Name+cityName+district.Name] = district.Adcode
				w.adcodeMap[province.Name+city.Name+district.Name] = district.Adcode
			}
		}
	}
}

// normalizeProvinceCityName removes suffixes only for provinces and cities
func (w *WeatherService) normalizeProvinceCityName(name string) string {
	suffixes := []string{"省", "市", "自治区", "特别行政区"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix)
		}
	}
	return name
}

func (w *WeatherService) getAdcode(location string) string {
	// Initialize city data if not already done
	w.initCityData()

	// Clean the input location string
	location = strings.TrimSpace(location)

	// Remove parentheses and their content (like "(市)" or "(省)")
	re := regexp.MustCompile(`[\(（][^)）]*[\)）]`)
	location = re.ReplaceAllString(location, "")

	// Try exact match first
	if adcode, exists := w.adcodeMap[location]; exists {
		return adcode
	}
	return ""

}
func (w *WeatherService) GetWeather(ctx context.Context, location string) ([]WeatherLive, error) {
	if w.cfg.Key == "" {
		return nil, fmt.Errorf("weather service key is not set")
	}
	var report WeatherData
	adcode := w.getAdcode(location)
	if adcode == "" {
		return nil, fmt.Errorf("no matching city found for %s", location)
	}
	logger.Info("Probe adcode from location", slog.String("location", location), slog.String("adcode", adcode))
	ret, err := w.client.R().
		SetContext(ctx).
		SetResult(&report).
		SetQueryParam("key", w.cfg.Key).
		SetQueryParam("city", adcode).
		Get("https://restapi.amap.com/v3/weather/weatherInfo")
	if err != nil {
		return nil, err
	}

	if ret.StatusCode() != 200 {
		return nil, fmt.Errorf("error fetching weather data: %s", ret.Status())
	}
	if report.Status != "1" {
		return nil, fmt.Errorf("error fetching weather data: %s", report.Info)
	}
	return report.Lives, nil
}
