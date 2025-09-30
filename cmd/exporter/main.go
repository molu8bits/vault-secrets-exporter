// cmd/exporter/main.go

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/molu8bits/vault-secrets-exporter/internal/exporter"

	"github.com/hashicorp/vault/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

// login handles the authentication logic for Vault.
// It prioritizes Token Auth, then falls back to AppRole Auth.
func login(ctx context.Context, client *api.Client) error {
	// Priority 1: Token Authentication
	vaultToken := os.Getenv("VAULT_TOKEN")
	if vaultToken != "" {
		log.Info("Authenticating using VAULT_TOKEN.")
		client.SetToken(vaultToken)
		// Verify the token is valid by looking up its capabilities
		_, err := client.Auth().Token().LookupSelf()
		if err != nil {
			return fmt.Errorf("VAULT_TOKEN is invalid or expired: %w", err)
		}
		log.Info("VAULT_TOKEN is valid.")
		return nil
	}

	// Priority 2: AppRole Authentication
	roleID := os.Getenv("VAULT_ROLE_ID")
	secretID := os.Getenv("VAULT_SECRET_ID")
	if roleID != "" && secretID != "" {
		log.Info("VAULT_TOKEN not found. Authenticating using AppRole.")

		// --- CORRECTED APPROLE LOGIC ---
		data := map[string]interface{}{
			"role_id":   roleID,
			"secret_id": secretID,
		}

		secret, err := client.Logical().Write("auth/approle/login", data)
		if err != nil {
			return fmt.Errorf("failed to login with AppRole: %w", err)
		}
		if secret == nil || secret.Auth == nil || secret.Auth.ClientToken == "" {
			return fmt.Errorf("no auth info was returned after AppRole login")
		}

		// Set the token for all subsequent requests
		client.SetToken(secret.Auth.ClientToken)
		log.Info("AppRole authentication successful.")
		// --- END CORRECTED LOGIC ---

		return nil
	}

	return fmt.Errorf("no authentication method configured. Please set VAULT_TOKEN or both VAULT_ROLE_ID and VAULT_SECRET_ID")
}

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	log.Info("Starting Vault Secrets Exporter")

	// --- Standard Configuration ---
	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		log.Fatal("VAULT_ADDR environment variable is required")
	}
	kvMountPath := os.Getenv("KV_MOUNT_PATH")
	if kvMountPath == "" {
		kvMountPath = "secret"
		log.Infof("KV_MOUNT_PATH not set, using default: %s", kvMountPath)
	}

	// --- Client Initialization ---
	config := &api.Config{
		Address: vaultAddr,
	}
	client, err := api.NewClient(config)
	if err != nil {
		log.Fatalf("Error creating Vault client: %v", err)
	}

	// --- Handle Vault Namespace ---
	vaultNamespace := os.Getenv("VAULT_NAMESPACE")
	if vaultNamespace != "" {
		client.SetNamespace(vaultNamespace)
		log.Infof("Using Vault Enterprise namespace: %s", vaultNamespace)
	}

	// --- Perform Authentication ---
	if err := login(context.Background(), client); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	// --- Exporter and Server Setup ---
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
