# `mqttwxenrich`

Enrich MQTT messages from weather sensors with unit conversions and supplemental calculations.

`mqttwxenrich` subscribes to an MQTT topic carrying JSON messages from [rtl_433](https://github.com/merbanan/rtl_433), enriches them with derived weather metrics and unit conversions using [libwx](https://github.com/cdzombak/libwx), and publishes the enriched data to a new MQTT topic. The enriched output uses `f_`/`t_` prefixed field and tag names for easy compatibility with [mqtt2influxdb](https://github.com/cdzombak/mqtt2influxdb).

## Supported Sensors

### Acurite 6045M

Temperature, humidity, and lightning sensor. Enrichment output includes:

| Output Field | Description |
|---|---|
| `time` | Timestamp from the original message |
| `t_model` | Sensor model name (tag) |
| `t_id` | Sensor ID (tag) |
| `f_temp_f` | Temperature in Fahrenheit |
| `f_temp_c` | Temperature in Celsius |
| `f_rel_humidity` | Relative humidity (%) |
| `f_abs_humidity` | Absolute humidity (g/m^3) |
| `f_dew_point_f` | Dew point in Fahrenheit |
| `f_dew_point_c` | Dew point in Celsius |
| `f_recommended_max_indoor_humidity` | Recommended maximum indoor humidity (%) |
| `f_wet_bulb_f` | Wet bulb temperature in Fahrenheit (when calculable) |
| `f_wet_bulb_c` | Wet bulb temperature in Celsius (when calculable) |
| `f_heat_index_f` | Heat index in Fahrenheit (when calculable) |
| `f_heat_index_c` | Heat index in Celsius (when calculable) |
| `f_storm_distance_mi` | Lightning storm distance in miles (when storm detected) |
| `f_storm_distance_km` | Lightning storm distance in kilometers (when storm detected) |

### Vevor 7-in-1

7-in-1 weather station. Enrichment output includes:

| Output Field | Description |
|---|---|
| `time` | Timestamp from the original message |
| `t_model` | Sensor model name (tag) |
| `t_id` | Sensor ID (tag) |
| `f_temp_f` | Temperature in Fahrenheit |
| `f_temp_c` | Temperature in Celsius |
| `f_rel_humidity` | Relative humidity (%) |
| `f_abs_humidity` | Absolute humidity (g/m^3) |
| `f_dew_point_f` | Dew point in Fahrenheit |
| `f_dew_point_c` | Dew point in Celsius |
| `f_recommended_max_indoor_humidity` | Recommended maximum indoor humidity (%) |
| `f_wet_bulb_f` | Wet bulb temperature in Fahrenheit (when calculable) |
| `f_wet_bulb_c` | Wet bulb temperature in Celsius (when calculable) |
| `f_heat_index_f` | Heat index in Fahrenheit (when calculable) |
| `f_heat_index_c` | Heat index in Celsius (when calculable) |
| `f_wind_chill_f` | Wind chill in Fahrenheit (when calculable) |
| `f_wind_chill_c` | Wind chill in Celsius (when calculable) |
| `f_wind_bearing` | Wind direction in degrees |
| `f_wind_speed_mph` | Wind speed in mph |
| `f_wind_speed_kmh` | Wind speed in km/h |
| `f_wind_speed_kt` | Wind speed in knots |
| `f_wind_gust_mph` | Wind gust speed in mph |
| `f_wind_gust_kmh` | Wind gust speed in km/h |
| `f_wind_gust_kt` | Wind gust speed in knots |
| `f_rain_cm` | Rainfall in centimeters |
| `f_rain_in` | Rainfall in inches |

## Usage

```text
mqttwxenrich [options]
```

`mqttwxenrich` expects JSON messages from [rtl_433](https://github.com/merbanan/rtl_433) using the flags `-F json -M time:iso:utc:tz`. Each message must include a `model` field matching a supported sensor model name (`Acurite-6045M` or `Vevor-7in1`). Messages with unsupported model names are logged and skipped.

Enriched messages are published to `<mqtt-topic>/enrichment`.

### Arguments

All arguments can also be specified via the corresponding environment variable.

| Flag | Env Var | Description | Required | Default |
|---|---|---|---|---|
| `-mqtt-server` | `MQTT_SERVER` | MQTT broker URL. The `mqtt://` scheme prefix is added automatically if not present. | **Yes** | -- |
| `-mqtt-topic` | `MQTT_TOPIC` | MQTT topic to subscribe to. Enriched output is written to `<this topic>/enrichment`. | **Yes** | -- |
| `-mqtt-user` | `MQTT_USER` | MQTT username for authentication. Required if `-mqtt-pass` is specified. | No | -- |
| `-mqtt-pass` | `MQTT_PASS` | MQTT password for authentication. Required if `-mqtt-user` is specified. | No | -- |
| `-mqtt-client-id` | `MQTT_CLIENT_ID` | MQTT client ID. If not specified, a random ID including the hostname and program name is generated. | No | Auto-generated |
| `-health-port` | `HEALTH_PORT` | Port on which to serve a health check HTTP endpoint. If not specified, no health endpoint is served. | No | -- |
| `-healthy-interval` | `HEALTHY_INTERVAL` | Interval (in seconds) at which messages must be received and enriched to be considered healthy. | No | `300` (5 minutes) |
| `-version` | -- | Print version and exit. | -- | -- |

### Docker

Docker images are available from both Docker Hub and GHCR. Configuration is done through environment variables.

```yaml
# docker-compose.yml example:
services:
  mqttwxenrich:
    image: ghcr.io/cdzombak/mqttwxenrich:latest
    environment:
      MQTT_SERVER: mqtt://my-broker:1883
      MQTT_TOPIC: home/weather/my-sensor
      MQTT_USER: myuser
      MQTT_PASS: mypassword
    healthcheck:
      test: ["CMD", "curl", "-sf", "http://localhost:8888/"]
      interval: 60s
      timeout: 5s
      retries: 3
      start_period: 10s
```

> [!NOTE]
> The Docker image sets `HEALTH_PORT=8888` by default and includes `curl` for health checking.

### Home Assistant

Example Home Assistant MQTT sensor configurations are included in the repository:

- [`home-assistant-acurite-6045m.yml`](home-assistant-acurite-6045m.yml) -- Acurite 6045M temperature, humidity, dew point, wet bulb, heat index, and lightning distance sensors.
- [`home-assistant-vevor-7in1.example.yml`](home-assistant-vevor-7in1.example.yml) -- Vevor 7-in-1 temperature, humidity, dew point, wet bulb, heat index, wind chill, wind speed/gust/direction, and rain sensors.

## Installation

### macOS via Homebrew

```shell
brew install cdzombak/oss/mqttwxenrich
```

### Debian via apt repository

[Install my Debian repository](https://www.dzombak.com/blog/2025/06/updated-instructions-for-installing-my-debian-package-repositories/) if you haven't already:

```shell
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://dist.cdzombak.net/keys/dist-cdzombak-net.gpg -o /etc/apt/keyrings/dist-cdzombak-net.gpg
sudo chmod 644 /etc/apt/keyrings/dist-cdzombak-net.gpg
sudo mkdir -p /etc/apt/sources.list.d
sudo curl -fsSL https://dist.cdzombak.net/cdzombak-oss.sources -o /etc/apt/sources.list.d/cdzombak-oss.sources
sudo chmod 644 /etc/apt/sources.list.d/cdzombak-oss.sources
sudo apt update
```

Then install `mqttwxenrich` via `apt-get`:

```shell
sudo apt-get install mqttwxenrich
```

### Manual installation from build artifacts

Pre-built binaries for Linux and macOS on various architectures are downloadable from each [GitHub Release](https://github.com/cdzombak/mqttwxenrich/releases). Debian packages for each release are available as well.

### Build and install locally

```shell
git clone https://github.com/cdzombak/mqttwxenrich.git
cd mqttwxenrich
make build

cp out/mqttwxenrich $INSTALL_DIR
```

## License

This software is licensed under the LGPL-3.0 license. See [LICENSE](LICENSE) in this repo.

## Author

Chris Dzombak
- [dzombak.com](https://dzombak.com)
- [GitHub @cdzombak](https://www.github.com/cdzombak)
