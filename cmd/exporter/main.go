package main

import (
	"net/http"
	"os"

	"github.com/molu8bits/vault-secrets-exporter/internal/exporter"

	"github.com/hashicorp/vault/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	log.Info("Starting Vault Secrets Exporter")

	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		log.Fatal("VAULT_ADDR environment variable is required")
	}
	vaultToken := os.Getenv("VAULT_TOKEN")
	if vaultToken == "" {
		log.Fatal("VAULT_TOKEN environment variable is required")
	}
	kvMountPath := os.Getenv("KV_MOUNT_PATH")
	if kvMountPath == "" {
		kvMountPath = "secret" // Default for KV v2 engine
		log.Infof("KV_MOUNT_PATH not set, using default: %s", kvMountPath)
	}

	config := &api.Config{
		Address: vaultAddr,
	}
	client, err := api.NewClient(config)
	if err != nil {
		log.Fatalf("Error creating Vault client: %v", err)
	}
	client.SetToken(vaultToken)

	// Handle Vault Namespace ---
	vaultNamespace := os.Getenv("VAULT_NAMESPACE")
	if vaultNamespace != "" {
		client.SetNamespace(vaultNamespace)
		log.Infof("Using Vault Enterprise namespace: %s", vaultNamespace)
	}
	//

	exp := exporter.NewExporter(client, kvMountPath)
	prometheus.MustRegister(exp)

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Vault Secrets Exporter</title></head>
             <body>
             <h1>Vault Secrets Exporter</h1>
             <p><a href='/metrics'>Metrics</a></p>
             </body>
             </html>`))
	})

	log.Info("Server listening on port :9102")
	log.Fatal(http.ListenAndServe(":9102", nil))
}
