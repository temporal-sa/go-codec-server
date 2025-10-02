package main

import (
	"context"
	"crypto/tls"
	"log"
	"os"

	temporalconverters "go-codec-server"

	"go.temporal.io/sdk/client"
)

func main() {
	tlsCertPath := getEnv("TLS_CERT_PATH", "")
	tlsKeyPath := getEnv("TLS_KEY_PATH", "")
	hostPort := getEnv("TEMPORAL_HOST_PORT", "localhost:7233")
	namespace := getEnv("TEMPORAL_NAMESPACE", "default")
	cert, err := tls.LoadX509KeyPair(tlsCertPath, tlsKeyPath)
	if err != nil {
		log.Fatalln("Unable to load cert and key pair", err)
	}
	// The client is a heavyweight object that should be created once per process.
	c, err := client.Dial(client.Options{
		// Set DataConverter here to ensure that workflow inputs and results are
		// encoded as required.
		DataConverter: temporalconverters.DataConverter,
		HostPort:      hostPort,
		Namespace:     namespace,
		ConnectionOptions: client.ConnectionOptions{
			TLS: &tls.Config{
				Certificates: []tls.Certificate{cert},
			},
		},
	})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	workflowOptions := client.StartWorkflowOptions{
		ID:        "converters_workflowID",
		TaskQueue: "converters",
	}

	we, err := c.ExecuteWorkflow(
		context.Background(),
		workflowOptions,
		temporalconverters.Workflow,
		"Plain text input",
	)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	log.Println("Started workflow", "WorkflowID", we.GetID(), "RunID", we.GetRunID())

	// Synchronously wait for the workflow completion.
	var result string
	err = we.Get(context.Background(), &result)
	if err != nil {
		log.Fatalln("Unable get workflow result", err)
	}
	log.Println("Workflow result:", result)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
