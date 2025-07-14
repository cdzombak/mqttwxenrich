package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	ec "github.com/cdzombak/exitcode_go"
	"github.com/cdzombak/heartbeat"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

const (
	EnvVarMqttServer   = "MQTT_SERVER"
	EnvVarMqttUser     = "MQTT_USER"
	EnvVarMqttPass     = "MQTT_PASS"
	EnvVarMqttTopic    = "MQTT_TOPIC"
	EnvVarMqttClientID = "MQTT_CLIENT_ID"
	HealthPort         = "HEALTH_PORT"
	HealthyInterval    = "HEALTHY_INTERVAL"
)

type Config struct {
	MqttServer      *url.URL
	MqttUser        string
	MqttPass        string
	MqttTopic       string
	DestTopic       string
	MqttClientID    string
	HealthPort      int
	HealthyInterval time.Duration
}

func Main(cfg *Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var (
		hb  heartbeat.Heartbeat
		err error
	)
	if cfg.HealthPort > 0 {
		hb, err = heartbeat.NewHeartbeat(&heartbeat.Config{
			HeartbeatInterval: 2 * cfg.HealthyInterval,
			LivenessThreshold: cfg.HealthyInterval,
			Port:              cfg.HealthPort,
		})
		if err != nil {
			log.Fatalf("failed to start heartbeat: %s", err)
		}
		hb.Start()
	}

	receivedMessages := make(chan paho.PublishReceived)

	cliCfg := autopaho.ClientConfig{
		ServerUrls:      []*url.URL{cfg.MqttServer},
		ConnectUsername: cfg.MqttUser,
		ConnectPassword: []byte(cfg.MqttPass),
		OnConnectionUp: func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
			// Subscribing in the OnConnectionUp callback is recommended (ensures the subscription is reestablished if the connection drops)
			log.Printf("connected to '%s'", cfg.MqttServer)

			subs := []paho.SubscribeOptions{{Topic: cfg.MqttTopic, QoS: 0}}
			if _, err := cm.Subscribe(ctx, &paho.Subscribe{Subscriptions: subs}); err != nil {
				log.Fatalf("failed to subscribe to topic '%s': %s", cfg.MqttTopic, err)
			}
			log.Printf("subscribed to topic '%s'", cfg.MqttTopic)
		},
		OnConnectError: func(err error) {
			log.Printf("error while attempting MQTT connection: %s", err)
		},
		// eclipse/paho.golang/paho provides base mqtt functionality, the below config will be passed in for each connection
		ClientConfig: paho.ClientConfig{
			ClientID: cfg.MqttClientID,
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				func(pr paho.PublishReceived) (bool, error) {
					receivedMessages <- pr
					return true, nil
				}},
			OnClientError: func(err error) {
				log.Fatalf("MQTT client error: %s", err)
			},
			OnServerDisconnect: func(d *paho.Disconnect) {
				if d.Properties != nil {
					log.Fatalf("MQTT server requested disconnect: %s\n", d.Properties.ReasonString)
				} else {
					log.Fatalf("MQTT server requested disconnect; reason code: %d\n", d.ReasonCode)
				}
			},
		},
	}
	c, err := autopaho.NewConnection(ctx, cliCfg)
	if err != nil {
		log.Fatalf("failed to start MQTT connection: %s", err)
	}

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case rm := <-receivedMessages:
				go handleMessage(ctx, c, cfg, hb, rm)
			}
		}
	}(ctx)

	<-c.Done()
	log.Println("signal caught - exiting")
	return nil
}

var version = "<dev>"

func main() {
	mqttServerStr := flag.String("mqtt-server", os.Getenv(EnvVarMqttServer),
		fmt.Sprintf("MQTT server. Defaults to env var %s. Required.", EnvVarMqttServer))
	mqttUser := flag.String("mqtt-user", os.Getenv(EnvVarMqttUser),
		fmt.Sprintf("MQTT user. Defaults to env var %s. Required iff -mqtt-pass is specified.", EnvVarMqttUser))
	mqttPass := flag.String("mqtt-pass", os.Getenv(EnvVarMqttPass),
		fmt.Sprintf("MQTT password. Defaults to env var %s.", EnvVarMqttPass))
	mqttTopic := flag.String("mqtt-topic", os.Getenv(EnvVarMqttTopic),
		fmt.Sprintf("MQTT topic on which to listen. Defaults to env var %s. Required. Output is written to <this topic>/enrichment.", EnvVarMqttTopic))
	mqttClientID := flag.String("mqtt-client-id", os.Getenv(EnvVarMqttClientID),
		fmt.Sprintf("MQTT client ID. Defaults to env var %s. If not specified, a random client ID including the hostname and program name is generated.", EnvVarMqttClientID))
	healthPortStr := flag.String("health-port", os.Getenv(HealthPort),
		fmt.Sprintf("Port on which to serve healthcheck endpoint. Defaults to env var %s. If not specified, no health endpoint is served.", HealthPort))
	printVersion := flag.Bool("version", false, "Print version and exit.")
	healthyIntervalStr := flag.String("healthy-interval", os.Getenv(HealthyInterval),
		fmt.Sprintf("Interval at which messages must be received from MQTT and enriched to be considered healthy (in seconds). Defaults to env var %s, or 300s (5 minutes) if unset.", HealthyInterval))
	flag.Parse()

	if *printVersion {
		fmt.Printf("mqttwxenrich %s\n", version)
		os.Exit(ec.Success)
	}

	var (
		mqttServer *url.URL
		err        error
	)
	if *mqttServerStr == "" {
		_, _ = fmt.Fprintf(os.Stderr, "MQTT server not specified. Must be given in %s or as -mqtt-server.\n", EnvVarMqttServer)
		os.Exit(ec.NotConfigured)
	} else {
		if !strings.HasPrefix(strings.ToLower(*mqttServerStr), "mqtt://") {
			*mqttServerStr = "mqtt://" + *mqttServerStr
		}
		mqttServer, err = url.Parse(*mqttServerStr)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to parse MQTT server URL '%s': %s\n", *mqttServerStr, err)
			os.Exit(ec.InvalidArgument)
		}
	}

	if *mqttTopic == "" {
		_, _ = fmt.Fprintf(os.Stderr, "MQTT topic not specified. Must be given in %s or as -mqtt-topic.\n", EnvVarMqttTopic)
		os.Exit(ec.NotConfigured)
	}
	if strings.HasSuffix(*mqttTopic, "/") {
		*mqttTopic = strings.TrimSuffix(*mqttTopic, "/")
	}

	hasUser := *mqttUser != ""
	hasPass := *mqttPass != ""
	if hasUser != hasPass {
		_, _ = fmt.Fprintf(os.Stderr, "MQTT user and password must both be specified, or neither.\n")
		os.Exit(ec.NotConfigured)
	}

	cfg := &Config{
		MqttClientID: *mqttClientID,
		MqttServer:   mqttServer,
		MqttUser:     *mqttUser,
		MqttPass:     *mqttPass,
		MqttTopic:    *mqttTopic,
		DestTopic:    fmt.Sprintf("%s/enrichment", *mqttTopic),
	}

	if *healthPortStr != "" {
		cfg.HealthPort, err = strconv.Atoi(*healthPortStr)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to parse health port '%s': %s\n", *healthPortStr, err)
			os.Exit(ec.InvalidArgument)
		}
		if cfg.HealthPort < 1 || cfg.HealthPort > 65535 {
			_, _ = fmt.Fprintf(os.Stderr, "Invalid health port '%s'.\n", *healthPortStr)
			os.Exit(ec.InvalidArgument)
		}
	}

	if *healthyIntervalStr == "" {
		cfg.HealthyInterval = 300 * time.Second
	} else {
		hi, err := strconv.Atoi(*healthyIntervalStr)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to parse healthy interval '%s': %s\n", *healthyIntervalStr, err)
		}
		if hi < 1 {
			_, _ = fmt.Fprintf(os.Stderr, "Invalid healthy interval '%s'.\n", *healthyIntervalStr)
		}
		cfg.HealthyInterval = time.Duration(hi) * time.Second
	}

	if cfg.MqttClientID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatalf("failed to get hostname: %s", err)
		}
		cfg.MqttClientID = fmt.Sprintf("%s-mqttwxenrich-%s", hostname, RandAlnumString(8))
		log.Printf("generated MQTT client ID: %s", cfg.MqttClientID)
	}

	if err := Main(cfg); err != nil {
		log.Fatalln(err.Error())
	}
}
