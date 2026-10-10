package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

func coords(lat, lon float64) string {
	// The API rejects more than 4 decimal places (it redirects to a
	// rounded URL), so round up front.
	return fmt.Sprintf("%.4f,%.4f", lat, lon)
}

// ResolvePoint looks up the forecast/zone/station URLs for a location, plus
// its list of observation stations.
func (c *Client) ResolvePoint(ctx context.Context, lat, lon float64) (*Point, error) {
	var pr pointResp
	if err := c.getJSON(ctx, "/points/"+coords(lat, lon), &pr); err != nil {
		return nil, err
	}
	p := pr.Properties
	pt := &Point{
		Forecast:            p.Forecast,
		ForecastHourly:      p.ForecastHourly,
		ForecastZone:        p.ForecastZone,
		County:              p.County,
		ObservationStations: p.ObservationStations,
		City:                p.RelativeLocation.Properties.City,
		State:               p.RelativeLocation.Properties.State,
	}
	if pt.ObservationStations != "" {
		var sr stationsResp
		if err := c.getJSON(ctx, pt.ObservationStations, &sr); err != nil {
			return nil, fmt.Errorf("stations: %w", err)
		}
		for _, f := range sr.Features {
			pt.Stations = append(pt.Stations, Station{ID: f.Properties.StationIdentifier, Name: f.Properties.Name})
		}
	}
	return pt, nil
}

// ActiveAlerts returns active alerts covering a point.
func (c *Client) ActiveAlerts(ctx context.Context, lat, lon float64) ([]Alert, error) {
	var ac AlertCollection
	if err := c.getJSON(ctx, "/alerts/active?point="+url.QueryEscape(coords(lat, lon)), &ac); err != nil {
		return nil, err
	}
	return ac.Alerts(), nil
}

// LatestObservation fetches the latest observation from the point's first
// station. If that station has no current observation (404) or its latest
// report has no temperature, it tries the next nearest, up to five
// stations, returning the best it found.
func (c *Client) LatestObservation(ctx context.Context, p *Point) (*Observation, error) {
	if len(p.Stations) == 0 {
		return nil, errors.New("no observation stations")
	}
	var first *Observation
	var lastErr error
	for i, st := range p.Stations {
		if i >= 5 {
			break
		}
		obs, err := c.observation(ctx, st.ID)
		if err == nil {
			if obs.TempF != nil {
				return obs, nil
			}
			if first == nil {
				first = obs
			}
			continue
		}
		lastErr = err
		var he *HTTPError
		if !errors.As(err, &he) || he.Status != 404 {
			break
		}
	}
	if first != nil {
		return first, nil
	}
	return nil, lastErr
}

func (c *Client) observation(ctx context.Context, stationID string) (*Observation, error) {
	var or observationResp
	if err := c.getJSON(ctx, "/stations/"+url.PathEscape(stationID)+"/observations/latest", &or); err != nil {
		return nil, err
	}
	p := or.Properties
	o := &Observation{
		StationID:    p.StationID,
		StationName:  p.StationName,
		Time:         p.Timestamp,
		Description:  strings.TrimSpace(p.TextDescription),
		TempF:        tempF(p.Temperature),
		DewpointF:    tempF(p.Dewpoint),
		Humidity:     p.RelativeHumidity.Value,
		WindDirDeg:   p.WindDirection.Value,
		WindMph:      speedMph(p.WindSpeed),
		GustMph:      speedMph(p.WindGust),
		PressureInHg: pressureInHg(p.BarometricPressure),
		VisibilityMi: distanceMi(p.Visibility),
		Precip3hIn:   lengthIn(p.Precip3h),
	}
	if o.PressureInHg == nil {
		o.PressureInHg = pressureInHg(p.SeaLevelPressure)
	}
	// Feels-like: heat index when warm, wind chill when cold, else temp.
	switch {
	case p.HeatIndex.Value != nil:
		o.FeelsLikeF = tempF(p.HeatIndex)
	case p.WindChill.Value != nil:
		o.FeelsLikeF = tempF(p.WindChill)
	default:
		o.FeelsLikeF = o.TempF
	}
	return o, nil
}

// Forecast fetches 12-hour periods (forecast) or hourly periods
// (forecastHourly) from the given URL, returning at most max periods.
func (c *Client) Forecast(ctx context.Context, forecastURL string, max int) ([]Period, error) {
	if forecastURL == "" {
		return nil, errors.New("no forecast URL for this point")
	}
	var fr forecastResp
	if err := c.getJSON(ctx, forecastURL, &fr); err != nil {
		return nil, err
	}
	var out []Period
	for _, p := range fr.Properties.Periods {
		if len(out) >= max {
			break
		}
		out = append(out, Period{
			Number:        p.Number,
			Name:          p.Name,
			Start:         p.StartTime,
			End:           p.EndTime,
			IsDaytime:     p.IsDaytime,
			Temperature:   p.Temperature,
			TempUnit:      p.TemperatureUnit,
			PrecipChance:  p.ProbabilityOfPrecipitation.Value,
			WindSpeed:     p.WindSpeed,
			WindDirection: p.WindDirection,
			Short:         p.ShortForecast,
			Detailed:      p.DetailedForecast,
		})
	}
	return out, nil
}
