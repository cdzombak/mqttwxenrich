package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/cdzombak/libwx"
	"github.com/eclipse/paho.golang/paho"
)

// acurite6045mMessage represents a JSON message from an Acurite 6045M
// temperature/humidity/lightning sensor, as published by rtl_433 given
// the flags `-F json -M time:iso:utc:tz`.
// Only the fields we need for enrichment are included.
type acurite6045mMessage struct {
	Humidity  float64 `json:"humidity"`
	StormDist int     `json:"storm_distance"`
	TempF     float64 `json:"temperature_F"`

	Time  time.Time `json:"time"`
	Model string    `json:"model"`
	ID    int       `json:"id"`
}

func enrichAcurite6045M(rm paho.PublishReceived) map[string]any {
	var msg acurite6045mMessage
	err := json.Unmarshal(rm.Packet.Payload, &msg)
	if err != nil {
		log.Printf("failed to unmarshal Acurite6045M message as JSON: %s", err)
		return nil
	}

	retv := make(map[string]any)

	// for the Acurite 6045M, enrichment includes:
	// - time, model, ID
	// - storm_distance_mi, storm_distance_km
	// - temp_f and temp_c
	// - rel_humidity
	// - dew_point_f, dew_point_c
	// - recommended_max_indoor_humidity
	// - wet_bulb_f, wet_bulb_c
	// - heat_index_f, heat_index_c
	// TODO(cdzombak): absolute humidity ( https://github.com/cdzombak/mqttwxenrich/issues/2 ; libwx https://github.com/cdzombak/libwx/issues/4 )
	// schema to match https://github.com/cdzombak/openweather-influxdb-connector/blob/294c9b7c201fe303c70eb450d767e695b0f9b037/main.go#L187-L221 .
	// fields named with f_ prefix for easy compatibility with https://github.com/cdzombak/mqtt2influxdb ;
	// tags named with t_ prefix

	retv["time"] = msg.Time
	retv["t_model"] = msg.Model
	retv["t_id"] = msg.ID

	stormDistMi := acuriteStormDistanceMi(msg.StormDist)
	stormDistKm := libwx.Mile(stormDistMi).Km()
	if stormDistMi == -1 {
		stormDistKm = -1
	}
	retv["f_storm_distance_mi"] = stormDistMi
	retv["f_storm_distance_km"] = stormDistKm

	retv["f_temp_f"] = msg.TempF
	retv["f_temp_c"] = libwx.TempF(msg.TempF).C().Unwrap()
	retv["f_rel_humidity"] = msg.Humidity
	retv["f_dew_point_f"] = libwx.DewPointF(libwx.TempF(msg.TempF), libwx.RelHumidity(msg.Humidity)).Unwrap()
	retv["f_dew_point_c"] = libwx.DewPointC(libwx.TempF(msg.TempF).C(), libwx.RelHumidity(msg.Humidity)).Unwrap()
	retv["f_recommended_max_indoor_humidity"] = libwx.IndoorHumidityRecommendationF(libwx.TempF(msg.TempF)).Unwrap()

	wetBulbTempF, wetBulbTempFErr := libwx.WetBulbF(libwx.TempF(msg.TempF), libwx.RelHumidity(msg.Humidity))
	wetBulbTempC, wetBulbTempCErr := libwx.WetBulbC(libwx.TempF(msg.TempF).C(), libwx.RelHumidity(msg.Humidity))
	if wetBulbTempFErr == nil {
		retv["f_wet_bulb_f"] = wetBulbTempF.Unwrap()
	}
	if wetBulbTempCErr == nil {
		retv["f_wet_bulb_c"] = wetBulbTempC.Unwrap()
	}

	heatIdxF, heatIdxFErr := libwx.HeatIndexFWithValidation(libwx.TempF(msg.TempF), libwx.RelHumidity(msg.Humidity))
	heatIdxC, heatIdxCErr := libwx.HeatIndexCWithValidation(libwx.TempF(msg.TempF).C(), libwx.RelHumidity(msg.Humidity))
	if heatIdxFErr == nil {
		retv["f_heat_index_f"] = heatIdxF.Unwrap()
	}
	if heatIdxCErr == nil {
		retv["f_heat_index_c"] = heatIdxC.Unwrap()
	}

	return retv
}

func acuriteStormDistanceMi(stormDist int) float64 {
	if stormDist < 0 || stormDist > 30 {
		return -1
	}

	// based on https://www.wxforum.net/index.php?topic=34293.msg409829#msg409829 :
	return map[int]float64{
		0:  1,
		1:  1.5,
		2:  2,
		3:  2.5,
		4:  3,
		5:  4,
		6:  4.5,
		7:  5,
		8:  6,
		9:  6.5,
		10: 7,
		11: 8,
		12: 9,
		13: 9.67,
		14: 10.34,
		15: 11,
		16: 11.5,
		17: 12,
		18: 13,
		19: 14,
		20: 15,
		21: 16,
		22: 17,
		23: 18,
		24: 19,
		25: 20,
		26: 21,
		27: 22,
		28: 23,
		29: 24,
		30: 25,
	}[stormDist]
}
