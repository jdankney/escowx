# escowx

A btop-style terminal dashboard for live National Weather Service data around
Escondido, CA: current conditions, forecasts, and active alerts, with a
four-level threat mode per location.

Data comes only from [api.weather.gov](https://www.weather.gov/documentation/services-web-api) (no API key).

## Build

Requires Go 1.24+.

```sh
CGO_ENABLED=0 go build -o escowx .
go test ./...
```

## Run

```sh
./escowx                 # full-screen dashboard
./escowx --once          # plain-text summary, then exit (for scripts/cron)
./escowx --demo          # inject fake alerts to preview every mode
./escowx --config ./other.yaml --contact me@example.com
```

`--once` exits 0 on success and 1 if no location could be loaded. It can be
combined with `--demo`.

## Config

On first run `~/.config/escowx/config.yaml` is created with the default
locations (`$XDG_CONFIG_HOME` is respected; `--config` points elsewhere). Edit
it to add or remove places:

```yaml
contact: you@example.com      # sent in the User-Agent; NWS asks for real contact info
refresh:
  alerts: 60s
  forecast: 10m               # observations + forecasts
# log_file: ~/escowx.log      # default: ~/.local/state/escowx/escowx.log
bell: true                    # terminal bell on entering TAKE COVER
notify: true                  # desktop notification (notify-send / osascript)
locations:
  - { name: Escondido, lat: 33.1192, lon: -117.0864, primary: true }
  - { name: Julian,    lat: 33.0787, lon: -116.6020 }
```

The contact can also be set with `--contact` or `ESCOWX_CONTACT`. The location
marked `primary` gets the large panel.

## Threat modes

| Mode | Color | Triggered by (examples) |
|---|---|---|
| NORMAL | green | no alerts, Statements, Outlooks |
| WARNING | yellow | Advisories and Watches |
| HEIGHTENED WARNING | orange | Warnings (Red Flag, High Wind, Flash Flood, …) |
| TAKE COVER | red, flashing banner | Tornado / Extreme Wind / Tsunami Warning, Flash Flood or Tornado Emergency, civil emergencies, or any Extreme + Immediate alert |

The event→mode table lives in [`modes/events.go`](modes/events.go). Events
not in the table fall back to severity ("Severe" → HEIGHTENED), then wording
("…Warning", "…Watch", …). Each location shows its worst live alert; the header
shows the worst location overall. Mode changes are appended to
`~/.local/state/escowx/escowx.log` (`$XDG_STATE_HOME` is respected).

## Tabs

- **1 Main**: the primary location (or one you opened from another tab): current
  conditions, an hourly temperature/rain strip, and the detailed forecast.
- **2 Nearby**: cards for every other location: mode, temperature, conditions,
  wind, humidity, a 12-hour trend, and active alerts.
- **3 Map**: every location plotted at its real position over a colored field
  interpolated between them. Press `v` to switch layers: **temperature** (with
  wind arrows), **threat** (each area colored by its nearest location's mode),
  or **rain chance** this hour.

The alerts pane (every active alert, worst first) and the TAKE COVER banner
appear on every tab.

## Keys

| Key | Action |
|---|---|
| `q` / `ctrl+c` | quit |
| `r` | refresh everything now |
| `1` `2` `3` / `tab` / `shift+tab` | switch tabs |
| arrows / `hjkl` | Nearby: move around the grid · Map: jump to the nearest place in that direction · ↓ past the bottom enters the alerts pane |
| `enter` | open the selected place on Main; in the alerts pane, open the full alert |
| `esc` | leave the alerts pane / return Main to the primary location / close |
| `a` | jump to the alerts pane |
| `v` | Map: cycle layers |
| `?` | help |
| `m` / `M` | demo mode only: cycle the current location's mode, or every location's |

## Behavior notes

- Each location's `/points` lookup and station list are fetched once at startup and cached.
- Requests time out, retry 5xx/429 with exponential backoff, and never blank
  the screen. If a refresh fails, the last good data stays up with a **STALE** marker.
- Observations come from the location's nearest station. If its latest report
  has no temperature (common with personal weather stations), the next nearest
  is tried, up to five. Sky text falls back to the current hourly forecast.

## Layout

```
api/      NWS client, types, unit conversion
modes/    alert → threat-mode classification (+ tests with sample alert JSON)
wx/       per-location state, fetch orchestration, alert merging
ui/       Bubble Tea model and views (mapview.go: the Map tab)
tracker/  mode-change log, bell, desktop notifications
demo/     fake alerts for --demo
config/   config.yaml loading/defaults
```
