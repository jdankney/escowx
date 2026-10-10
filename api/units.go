package api

import "strings"

// Unit conversions keyed on the WMO unit codes the API returns
// (e.g. "wmoUnit:degC"). Unknown units yield nil rather than a wrong number.

func unit(q quantity) string {
	u := q.UnitCode
	if i := strings.LastIndexByte(u, ':'); i >= 0 {
		u = u[i+1:]
	}
	return u
}

func conv(q quantity, f func(string, float64) (float64, bool)) *float64 {
	if q.Value == nil {
		return nil
	}
	v, ok := f(unit(q), *q.Value)
	if !ok {
		return nil
	}
	return &v
}

func tempF(q quantity) *float64 {
	return conv(q, func(u string, v float64) (float64, bool) {
		switch u {
		case "degC":
			return v*9/5 + 32, true
		case "degF":
			return v, true
		case "K":
			return (v-273.15)*9/5 + 32, true
		}
		return 0, false
	})
}

func speedMph(q quantity) *float64 {
	return conv(q, func(u string, v float64) (float64, bool) {
		switch u {
		case "km_h-1":
			return v * 0.621371, true
		case "m_s-1":
			return v * 2.236936, true
		case "kn":
			return v * 1.150779, true
		case "mi_h-1":
			return v, true
		}
		return 0, false
	})
}

func pressureInHg(q quantity) *float64 {
	return conv(q, func(u string, v float64) (float64, bool) {
		switch u {
		case "Pa":
			return v / 3386.389, true
		case "hPa":
			return v / 33.86389, true
		}
		return 0, false
	})
}

func distanceMi(q quantity) *float64 {
	return conv(q, func(u string, v float64) (float64, bool) {
		switch u {
		case "m":
			return v / 1609.344, true
		case "km":
			return v / 1.609344, true
		}
		return 0, false
	})
}

func lengthIn(q quantity) *float64 {
	return conv(q, func(u string, v float64) (float64, bool) {
		switch u {
		case "mm":
			return v / 25.4, true
		case "cm":
			return v / 2.54, true
		case "m":
			return v / 0.0254, true
		}
		return 0, false
	})
}

// Compass converts a bearing in degrees to a 16-point compass direction.
func Compass(deg float64) string {
	dirs := [...]string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	i := int((deg+11.25)/22.5) % 16
	if i < 0 {
		i += 16
	}
	return dirs[i]
}
