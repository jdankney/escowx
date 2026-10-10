package modes

// EventModes maps NWS alert event names (the alert's "event" field) to a
// threat mode. This is the one place to change how a specific alert is
// treated. Matching is case-insensitive.
//
// Events not listed here fall back to severity and wording: severity
// "Severe" → HEIGHTENED WARNING; otherwise "...Emergency" → TAKE COVER,
// "...Warning" → HEIGHTENED, "...Watch"/"...Advisory" → WARNING,
// "...Statement"/"...Outlook" → NORMAL; otherwise severity Minor/Moderate
// → WARNING, anything else → NORMAL.
//
// Listed events use the table as-is (NWS marks most watches "Severe", so
// severity alone would over-escalate them), except that any alert is
// raised to:
//   - TAKE COVER if its severity is "Extreme" and urgency is "Immediate"
//   - TAKE COVER if it carries a CATASTROPHIC damage threat (this is how
//     NWS marks Flash Flood Emergencies and Tornado Emergencies, which are
//     issued as ordinary Flash Flood / Tornado Warnings)
//
// Full list of event names: https://api.weather.gov/alerts/types
var EventModes = map[string]Mode{
	// ── TAKE COVER: act now to protect life ─────────────────────────────
	"Tornado Warning":             TakeCover,
	"Extreme Wind Warning":        TakeCover,
	"Tsunami Warning":             TakeCover,
	"Flash Flood Emergency":       TakeCover, // not a real event name today; see above
	"Fire Warning":                TakeCover,
	"Evacuation Immediate":        TakeCover,
	"Civil Danger Warning":        TakeCover,
	"Shelter In Place Warning":    TakeCover,
	"Nuclear Power Plant Warning": TakeCover,
	"Law Enforcement Warning":     TakeCover,
	"Radiological Hazard Warning": TakeCover,
	"Hazardous Materials Warning": TakeCover,

	// ── HEIGHTENED WARNING: hazardous weather occurring or imminent ─────
	"Red Flag Warning":            Heightened,
	"Excessive Heat Warning":      Heightened,
	"Extreme Heat Warning":        Heightened,
	"Heat Warning":                Heightened,
	"High Wind Warning":           Heightened,
	"Flash Flood Warning":         Heightened,
	"Flood Warning":               Heightened,
	"Severe Thunderstorm Warning": Heightened,
	"Winter Storm Warning":        Heightened,
	"Blizzard Warning":            Heightened,
	"Ice Storm Warning":           Heightened,
	"Dust Storm Warning":          Heightened,
	"High Surf Warning":           Heightened,
	"Coastal Flood Warning":       Heightened,
	"Hurricane Warning":           Heightened,
	"Tropical Storm Warning":      Heightened,
	"Storm Surge Warning":         Heightened,
	"Freeze Warning":              Heightened,
	"Hard Freeze Warning":         Heightened,
	"Extreme Cold Warning":        Heightened,
	"Gale Warning":                Heightened,
	"Storm Warning":               Heightened,
	"Earthquake Warning":          Heightened,
	"Volcano Warning":             Heightened,

	// ── WARNING: be aware, conditions may become hazardous ──────────────
	"Wind Advisory":             Warning,
	"Heat Advisory":             Warning,
	"Dense Fog Advisory":        Warning,
	"Dense Smoke Advisory":      Warning,
	"Air Quality Alert":         Warning,
	"Fire Weather Watch":        Warning,
	"Flood Watch":               Warning,
	"Flash Flood Watch":         Warning,
	"Flood Advisory":            Warning,
	"Excessive Heat Watch":      Warning,
	"Extreme Heat Watch":        Warning,
	"High Wind Watch":           Warning,
	"Tornado Watch":             Warning,
	"Severe Thunderstorm Watch": Warning,
	"Winter Storm Watch":        Warning,
	"Winter Weather Advisory":   Warning,
	"Frost Advisory":            Warning,
	"Freeze Watch":              Warning,
	"Blowing Dust Advisory":     Warning,
	"High Surf Advisory":        Warning,
	"Small Craft Advisory":      Warning,
	"Coastal Flood Advisory":    Warning,
	"Lake Wind Advisory":        Warning,
	"Air Stagnation Advisory":   Warning,
	"Tsunami Advisory":          Warning,
	"Tsunami Watch":             Warning,
	"Tropical Storm Watch":      Warning,
	"Hurricane Watch":           Warning,
	"Storm Surge Watch":         Warning,
	"Coastal Flood Watch":       Warning,

	// ── NORMAL: informational only ──────────────────────────────────────
	"Special Weather Statement":        Normal,
	"Hazardous Weather Outlook":        Normal,
	"Short Term Forecast":              Normal,
	"Severe Weather Statement":         Normal,
	"Flood Statement":                  Normal,
	"Flash Flood Statement":            Normal,
	"Hydrologic Outlook":               Normal,
	"Beach Hazards Statement":          Normal,
	"Rip Current Statement":            Normal,
	"Tropical Cyclone Local Statement": Normal,
	"Hurricane Local Statement":        Normal,
	"Test":                             Normal,
	"Administrative Message":           Normal,
}
