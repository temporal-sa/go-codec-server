package main

import (
	"crypto/tls"
	"log"
	"os"

	temporalconverters "go-codec-server"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	tlsCertPath := getEnv("TLS_CERT_PATH", "")
	tlsKeyPath := getEnv("TLS_KEY_PATH", "")
	hostPort := getEnv("TEMPORAL_HOST_PORT", "localhost:7233")
	namespace := getEnv("TEMPORAL_NAMESPACE", "default")

	log.Printf("Worker configuration:")
	log.Printf("  TLS Cert Path: %s", tlsCertPath)
	log.Printf("  TLS Key Path: %s", tlsKeyPath)
	log.Printf("  Host Port: %s", hostPort)
	log.Printf("  Namespace: %s", namespace)

	var connectopts client.ConnectionOptions

	if tlsCertPath == "" || tlsKeyPath == "" {
		connectopts = client.ConnectionOptions{}
	} else {
		cert, err := tls.LoadX509KeyPair(tlsCertPath, tlsKeyPath)
		if err != nil {
			log.Fatalln("Unable to load cert and key pair", err)
		}
		connectopts = client.ConnectionOptions{
			TLS: &tls.Config{
				Certificates: []tls.Certificate{cert},
			},
		}
	}

	c, err := client.Dial(client.Options{
		// Set DataConverter here so that workflow and activity inputs/results will
		// be compressed as required.
		HostPort:          hostPort,
		Namespace:         namespace,
		DataConverter:     temporalconverters.DataConverter,
		ConnectionOptions: connectopts,
	},
	)
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	w := worker.New(c, "converters", worker.Options{})

	w.RegisterWorkflow(temporalconverters.Workflow)
	w.RegisterActivity(temporalconverters.Activity)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
