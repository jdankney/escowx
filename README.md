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
./escowx --config ~/.config/escowx.yaml --contact me@example.com
```

`--once` exits 0 on success and 1 if no location could be loaded. It can be
combined with `--demo`.

## Config

On first run `config.yaml` is created in the current directory (or wherever
`--config` points) with the default locations. Edit it to add or remove places:

```yaml
contact: you@example.com      # sent in the User-Agent; NWS asks for real contact info
refresh:
  alerts: 60s
  forecast: 10m               # observations + forecasts
log_file: escowx.log          # relative to the config file
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
shows the worst location overall. Mode changes are appended to `escowx.log`.

## Keys

| Key | Action |
|---|---|
| `q` / `ctrl+c` | quit |
| `r` | refresh everything now |
| arrows / `hjkl` | move between the large panel, the grid, and the alerts pane |
| `tab` / `shift+tab` | cycle through locations, then the alerts pane |
| `enter` | show the selected location in the large panel; in the alerts pane, open the full alert |
| `esc` | back to the primary location / close |
| `a` | jump to the alerts pane |
| `?` | help |
| `m` / `M` | demo mode only: cycle the selected location's mode, or every location's |

## Behavior notes

- Each location's `/points` lookup and station list are fetched once at startup and cached.
- Requests time out, retry 5xx/429 with exponential backoff, and never blank
  the screen. If a refresh fails, the last good data stays up with a **STALE** marker.
- Observations come from the location's nearest station. Many of these are
  personal weather stations that don't report sky conditions, so the sky text
  falls back to the current hourly forecast.

## Layout

```
api/      NWS client, types, unit conversion
modes/    alert → threat-mode classification (+ tests with sample alert JSON)
wx/       per-location state, fetch orchestration, alert merging
ui/       Bubble Tea model and views
tracker/  mode-change log, bell, desktop notifications
demo/     fake alerts for --demo
config/   config.yaml loading/defaults
```
