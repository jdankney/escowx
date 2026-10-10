package api

import (
	"strings"
	"time"
)

// Point holds the per-location URLs resolved once from /points/{lat},{lon}.
type Point struct {
	Forecast            string
	ForecastHourly      string
	ForecastZone        string
	County              string
	ObservationStations string
	City, State         string
	// Stations are station identifiers from ObservationStations, nearest
	// first. Resolved alongside the point and cached with it.
	Stations []Station
}

type Station struct {
	ID   string
	Name string
}

// Observation is a station observation already converted to US units.
// Missing values are nil.
type Observation struct {
	StationID    string
	StationName  string
	Time         time.Time
	Description  string
	TempF        *float64
	DewpointF    *float64
	FeelsLikeF   *float64
	Humidity     *float64 // percent
	WindDirDeg   *float64
	WindMph      *float64
	GustMph      *float64
	PressureInHg *float64
	VisibilityMi *float64
	Precip3hIn   *float64
}

// Period is one forecast period (12-hourly or hourly).
type Period struct {
	Number        int
	Name          string
	Start, End    time.Time
	IsDaytime     bool
	Temperature   int
	TempUnit      string
	PrecipChance  *float64
	WindSpeed     string
	WindDirection string
	Short         string
	Detailed      string
}

// Alert is an active NWS alert.
type Alert struct {
	ID          string              `json:"id"`
	AreaDesc    string              `json:"areaDesc"`
	Sent        time.Time           `json:"sent"`
	Effective   time.Time           `json:"effective"`
	Onset       time.Time           `json:"onset"`
	Expires     time.Time           `json:"expires"`
	Ends        time.Time           `json:"ends"`
	Status      string              `json:"status"`
	MessageType string              `json:"messageType"`
	Category    string              `json:"category"`
	Severity    string              `json:"severity"`
	Certainty   string              `json:"certainty"`
	Urgency     string              `json:"urgency"`
	Event       string              `json:"event"`
	SenderName  string              `json:"senderName"`
	Headline    string              `json:"headline"`
	Description string              `json:"description"`
	Instruction string              `json:"instruction"`
	Response    string              `json:"response"`
	Parameters  map[string][]string `json:"parameters"`
}

// Param returns the first value of an alert parameter, or "".
func (a *Alert) Param(name string) string {
	if v := a.Parameters[name]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// EndTime is when the hazard ends: Ends if given, otherwise Expires.
func (a *Alert) EndTime() time.Time {
	if !a.Ends.IsZero() {
		return a.Ends
	}
	return a.Expires
}

// Live reports whether the alert should still be considered in effect at
// now: an actual (non-test) message that hasn't been cancelled or expired.
func (a *Alert) Live(now time.Time) bool {
	if a.Status != "" && !strings.EqualFold(a.Status, "Actual") {
		return false
	}
	if strings.EqualFold(a.MessageType, "Cancel") {
		return false
	}
	if end := a.EndTime(); !end.IsZero() && now.After(end) {
		return false
	}
	return true
}

// AlertCollection is the GeoJSON body of /alerts/active.
type AlertCollection struct {
	Features []struct {
		Properties Alert `json:"properties"`
	} `json:"features"`
}

// Alerts flattens the collection.
func (c *AlertCollection) Alerts() []Alert {
	out := make([]Alert, 0, len(c.Features))
	for _, f := range c.Features {
		out = append(out, f.Properties)
	}
	return out
}

// Raw API shapes used only for decoding.

type quantity struct {
	UnitCode string   `json:"unitCode"`
	Value    *float64 `json:"value"`
}

type pointResp struct {
	Properties struct {
		Forecast            string `json:"forecast"`
		ForecastHourly      string `json:"forecastHourly"`
		ForecastZone        string `json:"forecastZone"`
		County              string `json:"county"`
		ObservationStations string `json:"observationStations"`
		RelativeLocation    struct {
			Properties struct {
				City  string `json:"city"`
				State string `json:"state"`
			} `json:"properties"`
		} `json:"relativeLocation"`
	} `json:"properties"`
}

type stationsResp struct {
	Features []struct {
		Properties struct {
			StationIdentifier string `json:"stationIdentifier"`
			Name              string `json:"name"`
		} `json:"properties"`
	} `json:"features"`
}

type observationResp struct {
	Properties struct {
		StationID          string    `json:"stationId"`
		StationName        string    `json:"stationName"`
		Timestamp          time.Time `json:"timestamp"`
		TextDescription    string    `json:"textDescription"`
		Temperature        quantity  `json:"temperature"`
		Dewpoint           quantity  `json:"dewpoint"`
		WindDirection      quantity  `json:"windDirection"`
		WindSpeed          quantity  `json:"windSpeed"`
		WindGust           quantity  `json:"windGust"`
		BarometricPressure quantity  `json:"barometricPressure"`
		SeaLevelPressure   quantity  `json:"seaLevelPressure"`
		Visibility         quantity  `json:"visibility"`
		Precip3h           quantity  `json:"precipitationLast3Hours"`
		RelativeHumidity   quantity  `json:"relativeHumidity"`
		WindChill          quantity  `json:"windChill"`
		HeatIndex          quantity  `json:"heatIndex"`
	} `json:"properties"`
}

type forecastResp struct {
	Properties struct {
		Periods []struct {
			Number                     int       `json:"number"`
			Name                       string    `json:"name"`
			StartTime                  time.Time `json:"startTime"`
			EndTime                    time.Time `json:"endTime"`
			IsDaytime                  bool      `json:"isDaytime"`
			Temperature                int       `json:"temperature"`
			TemperatureUnit            string    `json:"temperatureUnit"`
			ProbabilityOfPrecipitation quantity  `json:"probabilityOfPrecipitation"`
			WindSpeed                  string    `json:"windSpeed"`
			WindDirection              string    `json:"windDirection"`
			ShortForecast              string    `json:"shortForecast"`
			DetailedForecast           string    `json:"detailedForecast"`
		} `json:"periods"`
	} `json:"properties"`
}
