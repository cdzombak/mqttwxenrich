package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/cdzombak/libwx"
	"github.com/eclipse/paho.golang/paho"
)

// vevor7in1Message represents a JSON message from a Vevor 7-in-1 weather
// station, as published by rtl_433 given the flags `-F json -M time:iso:utc:tz`.
// Only the fields we need for enrichment are included.
// nb. barometric pressure sensor is in the indoor display unit.
type vevor7in1Message struct {
	Humidity   float64 `json:"humidity"`
	RainMm     float64 `json:"rain_mm"`
	TempC      float64 `json:"temperature_C"`
	UV         int     `json:"uv"`
	WindAvgKmH float64 `json:"wind_avg_km_h"`
	WindDirDeg float64 `json:"wind_dir_deg"`
	WindMaxKmH float64 `json:"wind_max_km_h"`

	Time  time.Time `json:"time"`
	Model string    `json:"model"`
	ID    int       `json:"id"`
}

func enrichVevor7in1(rm paho.PublishReceived) map[string]any {
	var msg vevor7in1Message
	err := json.Unmarshal(rm.Packet.Payload, &msg)
	if err != nil {
		log.Printf("failed to unmarshal Vevor7in1 message as JSON: %s", err)
		return nil
	}

	retv := make(map[string]any)

	// for the Vevor 7-in-1 weather station, enrichment includes:
	// - time, model, ID
	// - rain_cm, rain_in
	// - temp_f and temp_c
	// - rel_humidity
	// - dew_point_f, dew_point_c
	// - wind_speed_mph, wind_speed_kmh, wind_speed_kt
	// - wind_gust_mph, wind_gust_kmh, wind_gust_kt
	// - wind_bearing (alias of wind_dir_deg)
	// - recommended_max_indoor_humidity
	// - wet_bulb_f, wet_bulb_c
	// - heat_index_f, heat_index_c
	// - wind_chill_f, wind_chill_c
	// TODO(cdzombak): absolute humidity ( https://github.com/cdzombak/mqttwxenrich/issues/2 ; libwx https://github.com/cdzombak/libwx/issues/4 )
	// schema to match https://github.com/cdzombak/openweather-influxdb-connector/blob/294c9b7c201fe303c70eb450d767e695b0f9b037/main.go#L187-L221 .
	// fields named with f_ prefix for easy compatibility with https://github.com/cdzombak/mqtt2influxdb ;
	// tags named with t_ prefix

	retv["time"] = msg.Time
	retv["t_model"] = msg.Model
	retv["t_id"] = msg.ID

	retv["f_temp_f"] = libwx.TempC(msg.TempC).F().Unwrap()
	retv["f_temp_c"] = msg.TempC
	retv["f_rel_humidity"] = msg.Humidity
	retv["f_dew_point_f"] = libwx.DewPointF(libwx.TempC(msg.TempC).F(), libwx.RelHumidity(msg.Humidity)).Unwrap()
	retv["f_dew_point_c"] = libwx.DewPointC(libwx.TempC(msg.TempC), libwx.RelHumidity(msg.Humidity)).Unwrap()
	retv["f_recommended_max_indoor_humidity"] = libwx.IndoorHumidityRecommendationF(libwx.TempC(msg.TempC).F()).Unwrap()

	wetBulbTempF, wetBulbTempFErr := libwx.WetBulbF(libwx.TempC(msg.TempC).F(), libwx.RelHumidity(msg.Humidity))
	wetBulbTempC, wetBulbTempCErr := libwx.WetBulbC(libwx.TempC(msg.TempC), libwx.RelHumidity(msg.Humidity))
	if wetBulbTempFErr == nil {
		retv["f_wet_bulb_f"] = wetBulbTempF.Unwrap()
	}
	if wetBulbTempCErr == nil {
		retv["f_wet_bulb_c"] = wetBulbTempC.Unwrap()
	}

	heatIdxF, heatIdxFErr := libwx.HeatIndexFWithValidation(libwx.TempC(msg.TempC).F(), libwx.RelHumidity(msg.Humidity))
	heatIdxC, heatIdxCErr := libwx.HeatIndexCWithValidation(libwx.TempC(msg.TempC), libwx.RelHumidity(msg.Humidity))
	if heatIdxFErr == nil {
		retv["f_heat_index_f"] = heatIdxF.Unwrap()
	}
	if heatIdxCErr == nil {
		retv["f_heat_index_c"] = heatIdxC.Unwrap()
	}

	retv["f_rain_cm"] = msg.RainMm / 10.0
	retv["f_rain_in"] = msg.RainMm / 25.4

	retv["f_wind_bearing"] = msg.WindDirDeg
	retv["f_wind_speed_mph"] = libwx.SpeedKmH(msg.WindAvgKmH).Mph().Unwrap()
	retv["f_wind_speed_kmh"] = msg.WindAvgKmH
	retv["f_wind_speed_kt"] = libwx.SpeedKmH(msg.WindAvgKmH).Knots().Unwrap()
	retv["f_wind_gust_mph"] = libwx.SpeedKmH(msg.WindMaxKmH).Mph().Unwrap()
	retv["f_wind_gust_kmh"] = msg.WindMaxKmH
	retv["f_wind_gust_kt"] = libwx.SpeedKmH(msg.WindMaxKmH).Knots().Unwrap()

	windChillF, windChillFErr := libwx.WindChillFWithValidation(libwx.TempC(msg.TempC).F(), libwx.SpeedKmH(msg.WindAvgKmH).Mph())
	windChillC, windChillCErr := libwx.WindChillCWithValidation(libwx.TempC(msg.TempC), libwx.SpeedKmH(msg.WindAvgKmH).Mph())
	if windChillFErr == nil {
		retv["f_wind_chill_f"] = windChillF.Unwrap()
	}
	if windChillCErr == nil {
		retv["f_wind_chill_c"] = windChillC.Unwrap()
	}

	return retv
}
