package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/cdzombak/heartbeat"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

const (
	ModelVevor7in1    = "Vevor-7in1"
	ModelAcurite6045M = "Acurite-6045M"
)

func handleMessage(
	ctx context.Context,
	mqtt *autopaho.ConnectionManager,
	cfg *Config,
	hb heartbeat.Heartbeat,
	msg paho.PublishReceived,
) {
	jsonMap := make(map[string]any)
	err := json.Unmarshal(msg.Packet.Payload, &jsonMap)
	if err != nil {
		log.Printf("failed to unmarshal message as JSON: %s", err)
		return
	}
	model, ok := jsonMap["model"]
	if !ok {
		log.Printf("message is missing 'model' field")
		return
	}
	modelStr, ok := model.(string)
	if !ok {
		log.Printf("message's 'model' field is not a string")
		return
	}

	var enrichmentMsg map[string]any
	switch modelStr {
	case ModelVevor7in1:
		enrichmentMsg = enrichVevor7in1(msg)
	case ModelAcurite6045M:
		enrichmentMsg = enrichAcurite6045M(msg)
	default:
		log.Printf("unsupported model: %s", modelStr)
	}

	if enrichmentMsg == nil {
		// assume the enricher logged some details
		return
	}

	enrichmentMsgBytes, err := json.Marshal(enrichmentMsg)
	if err != nil {
		log.Printf("failed to marshal enrichment message as JSON: %s", err)
		return
	}

	_, err = mqtt.Publish(ctx,
		&paho.Publish{
			QoS:     0,
			Retain:  false,
			Topic:   cfg.DestTopic,
			Payload: enrichmentMsgBytes,
		})
	if err != nil {
		log.Printf("failed to publish enrichment message: %s", err)
		return
	}

	if hb != nil {
		hb.Alive(time.Now())
	}
}
