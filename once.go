package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/j4mes/escowx/api"
	"github.com/j4mes/escowx/config"
	"github.com/j4mes/escowx/demo"
	"github.com/j4mes/escowx/modes"
	"github.com/j4mes/escowx/wx"
)

// runOnce fetches everything once, prints a plain-text summary and returns
// the process exit code: 0 on success, 1 if no location could be loaded.
func runOnce(cfg *config.Config, client *api.Client, demoMode bool, w io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	locs := make([]*wx.Location, len(cfg.Locations))
	var wg sync.WaitGroup
	for i, lc := range cfg.Locations {
		l := &wx.Location{Cfg: lc}
		locs[i] = l
		wg.Add(1)
		go func() {
			defer wg.Done()
			alerts, err := client.ActiveAlerts(ctx, lc.Lat, lc.Lon)
			now := time.Now()
			l.ApplyAlerts(alerts, err, now)
			l.Point, l.PointErr = client.ResolvePoint(ctx, lc.Lat, lc.Lon)
			if l.PointErr == nil {
				l.ApplyWeather(wx.FetchWeather(ctx, client, l.Point), time.Now())
			}
		}()
	}
	wg.Wait()

	now := time.Now()
	if demoMode {
		for i, m := range demo.Initial(len(locs)) {
			locs[i].DemoAlerts = demo.Alert(m, locs[i].Cfg.Name, now)
		}
	}
	for _, l := range locs {
		l.Reclassify(now)
	}

	overall, idx := wx.Overall(locs)
	fmt.Fprintf(w, "escowx — %s\n", now.Format("Mon Jan 2 2006 3:04 PM MST"))
	if idx >= 0 {
		cause := ""
		if c := locs[idx].Cause; c != nil {
			cause = " (" + c.Event + ")"
		}
		fmt.Fprintf(w, "Overall: %s — %s%s\n", overall, locs[idx].Cfg.Name, cause)
	} else {
		fmt.Fprintf(w, "Overall: %s\n", overall)
	}
	if demoMode {
		fmt.Fprintln(w, "*** DEMO MODE: alerts below include simulated ones ***")
	}

	loaded := 0
	fmt.Fprintln(w)
	for _, l := range locs {
		if l.Obs != nil || !l.AlertsUpdated.IsZero() {
			loaded++
		}
		fmt.Fprintf(w, "%-14s %-19s %s\n", l.Cfg.Name, "["+l.Mode.String()+"]", summary(l))
		if names := eventNames(l.AllAlerts(now)); names != "" {
			fmt.Fprintf(w, "%-14s %-19s alerts: %s\n", "", "", names)
		}
		for _, err := range []error{l.PointErr, l.WxErr, l.AlertsErr} {
			if err != nil {
				fmt.Fprintf(w, "%-14s %-19s error: %v\n", "", "", err)
			}
		}
	}

	p := locs[0]
	if len(p.Forecast) > 0 {
		fmt.Fprintf(w, "\n%s forecast\n", p.Cfg.Name)
		for _, f := range p.Forecast {
			pop := ""
			if f.PrecipChance != nil && *f.PrecipChance > 0 {
				pop = fmt.Sprintf(" (%.0f%% precip)", *f.PrecipChance)
			}
			fmt.Fprintf(w, "  %-16s %3d°%s  %s%s\n", f.Name, f.Temperature, f.TempUnit, f.Short, pop)
		}
	}
	if len(p.Hourly) > 0 {
		var parts []string
		for _, h := range p.Hourly {
			parts = append(parts, fmt.Sprintf("%s %d°", h.Start.Local().Format("3PM"), h.Temperature))
		}
		fmt.Fprintf(w, "\nNext %d hours: %s\n", len(p.Hourly), strings.Join(parts, " · "))
	}

	alerts := wx.MergeAlerts(locs, now)
	if len(alerts) > 0 {
		fmt.Fprintf(w, "\nActive alerts (%d)\n", len(alerts))
		for _, e := range alerts {
			end := "—"
			if t := e.Alert.EndTime(); !t.IsZero() {
				end = t.Local().Format("Mon 3:04 PM")
			}
			fmt.Fprintf(w, "  [%s] %s — until %s — %s\n", e.Mode, e.Alert.Event, end, strings.Join(e.Locations, ", "))
			if e.Alert.Headline != "" {
				fmt.Fprintf(w, "      %s\n", e.Alert.Headline)
			}
		}
	} else {
		fmt.Fprintln(w, "\nNo active alerts.")
	}

	if loaded == 0 {
		return 1
	}
	return 0
}

func summary(l *wx.Location) string {
	o := l.Obs
	if o == nil {
		return "—"
	}
	f := func(v *float64, format string) string {
		if v == nil {
			return "—"
		}
		return fmt.Sprintf(format, *v)
	}
	sky := o.Description
	if sky == "" && len(l.Hourly) > 0 {
		sky = l.Hourly[0].Short
	}
	if sky == "" {
		sky = "—"
	}
	wind := "—"
	if o.WindMph != nil && *o.WindMph < 0.5 {
		wind = "calm"
	} else if o.WindMph != nil {
		wind = fmt.Sprintf("%.0f mph", *o.WindMph)
		if o.WindDirDeg != nil {
			wind = api.Compass(*o.WindDirDeg) + " " + wind
		}
	}
	return fmt.Sprintf("%s (feels %s)  %s RH  wind %s  %s",
		f(o.TempF, "%.0f°F"), f(o.FeelsLikeF, "%.0f°F"), f(o.Humidity, "%.0f%%"), wind, sky)
}

func eventNames(alerts []api.Alert) string {
	seen := map[string]bool{}
	var out []string
	for _, a := range alerts {
		if !seen[a.Event] {
			seen[a.Event] = true
			out = append(out, fmt.Sprintf("%s [%s]", a.Event, modes.Classify(&a).Short()))
		}
	}
	return strings.Join(out, ", ")
}
