# Vault Secrets Exporter

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Helm Chart](https://img.shields.io/badge/Helm-v3-blue)](https://helm.sh)

A Prometheus exporter that scans HashiCorp Vault KV v2 secrets for specific metadata, exposing metrics about their expiration dates and configuration state. This tool helps you proactively manage the lifecycle of your static secrets. 🔑

---

## Overview

In a dynamic infrastructure, managing the lifecycle of static secrets (API keys, passwords, etc.) is a common challenge. While Vault is excellent for storing secrets, it doesn't natively provide a mechanism to monitor the expiration of static secrets stored in its KV engine.

This exporter solves that problem by:
1.  Recursively scanning a specified KV v2 mount path within a given Vault Namespace.
2.  Authenticating with Vault using either a Token or the more robust AppRole method.
3.  Reading the custom metadata of each secret.
4.  Exposing Prometheus metrics indicating when each secret expires.
5.  Highlighting secrets that are misconfigured (i.e., missing an expiry date).

This enables you to set up automated alerting and visualization to ensure no secret expires unexpectedly.

---

## Features

* **Recursive Secret Scanning:** Automatically discovers all secrets within a given KV v2 mount path.
* **Flexible Authentication:** Supports both Token and AppRole authentication methods.
* **Vault Enterprise Namespace Support:** Can operate within a specific Vault Enterprise namespace.
* **Expiration Monitoring:** Exposes the number of days until a secret expires as a Prometheus gauge.
* **Metadata as Labels:** Enriches metrics with `owner_email` and `usage_description` labels for better context.
* **Configuration Auditing:** Provides a specific metric for secrets that are missing `expiry_date` metadata.
* **Kubernetes Ready:** Includes a Dockerfile and a Helm chart for easy deployment.
* **Configurable:** All settings are managed via environment variables.

---

## Metrics Exposed

The exporter exposes the following Prometheus metrics on the `/metrics` endpoint.

| Metric Name                          | Type    | Description                                                                                          | Labels                                     |
| ------------------------------------ | ------- | ---------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| `vault_secret_expiry_days_remaining` | Gauge   | The number of days remaining until the secret expires. A negative value means the secret has expired.    | `path`, `owner_email`, `usage_description` |
| `vault_secret_has_no_expiry_date`    | Gauge   | Indicates if a secret is missing the `expiry_date` metadata. `1` means it's missing, `0` means it's present. | `path`                                     |
| `vault_exporter_scrape_errors_total` | Counter | The total number of errors encountered during a scrape operation.                                        | None                                       |

---

## Configuration

The exporter is configured using environment variables.

| Variable            | Required                                   | Default  | Description                                                                               |
| ------------------- | ------------------------------------------ | -------- | ----------------------------------------------------------------------------------------- |
| `VAULT_ADDR`        | **Yes** | `N/A`    | The full URL of the HashiCorp Vault server (e.g., `https://vault.example.com`).              |
| `KV_MOUNT_PATH`     | No                                         | `secret` | The mount path of the KV v2 secrets engine to be scanned.                                   |
| `VAULT_NAMESPACE`   | No                                         | `""`     | **(Enterprise Only)** The Vault namespace to operate within (e.g., `admin/my-team`).        |
| `VAULT_TOKEN`       | **Yes** (if not using AppRole)             | `""`     | The Vault token used for authentication.                                                  |
| `VAULT_ROLE_ID`     | **Yes** (if not using Token)               | `""`     | The RoleID for AppRole authentication.                                                    |
| `VAULT_SECRET_ID`   | **Yes** (if not using Token)               | `""`     | The SecretID for AppRole authentication.                                                  |

### Authentication Logic

The exporter uses the following authentication priority:
1.  If the `VAULT_TOKEN` environment variable is set, it will be used for token authentication.
2.  If `VAULT_TOKEN` is **not** set, the exporter will look for both `VAULT_ROLE_ID` and `VAULT_SECRET_ID` to perform an AppRole login.
3.  The application will fail to start if credentials for at least one of these methods are not provided.

### Vault Secret Metadata

For the exporter to work correctly, your secrets must have the following keys in their **custom metadata**:

* `expiry_date`: **(Required)** The expiration date in RFC3339 format (e.g., `2025-12-31T23:59:59Z`).
* `owner_email`: **(Optional)** The email of the person or team responsible for the secret.
* `usage_description`: **(Optional)** A short description of where the secret is used.

---

## Getting Started

### Prerequisites

* A running HashiCorp Vault instance (Community or Enterprise).
* A Kubernetes cluster (v1.19+).
* Helm v3 installed.
* Docker installed for building the image.

### Installation & Deployment

1.  **Clone the Repository**
    ```sh
    git clone [https://github.com/molu8bits/vault-secrets-exporter.git](https://github.com/molu8bits/vault-secrets-exporter.git)
    cd vault-secrets-exporter
    ```

2.  **Build and Push the Docker Image**
    ```sh
    # Replace 'your-repo' with your container registry username/organization
    docker build -t your-repo/vault-secrets-exporter:latest .
    docker push your-repo/vault-secrets-exporter:latest
    ```

3.  **Create Kubernetes Secrets for Authentication**

    Create **one** of the following secrets, depending on your chosen authentication method.

    * **For Token Authentication:**
        ```sh
        # Use your actual Vault token
        kubectl create secret generic vault-token --from-literal=token='s.YourSuperSecretVaultToken'
        ```

    * **For AppRole Authentication:**
        ```sh
        kubectl create secret generic vault-approle-credentials \
          --from-literal=role_id='YOUR_ROLE_ID_HERE' \
          --from-literal=secret_id='YOUR_SECRET_ID_HERE'
        ```

4.  **Deploy using the Helm Chart**

    Choose the command that matches your authentication method.

    * **Example: Deploying with Token Authentication**
        ```sh
        helm install my-exporter ./charts/vault-secrets-exporter \
          --namespace monitoring \
          --set image.repository="your-repo/vault-secrets-exporter" \
          --set vault.address="[https://vault.your-domain.com](https://vault.your-domain.com)" \
          --set auth.method="token" \
          --set auth.token.existingSecretName="vault-token"
        ```

    * **Example: Deploying with AppRole Authentication**
        ```sh
        helm install my-exporter ./charts/vault-secrets-exporter \
          --namespace monitoring \
          --set image.repository="your-repo/vault-secrets-exporter" \
          --set vault.address="[https://vault.your-domain.com](https://vault.your-domain.com)" \
          --set auth.method="approle" \
          --set auth.approle.existingSecretName="vault-approle-credentials" \
          --set vault.namespace="admin/my-team-space" # Optional: for Enterprise
        ```

---

## Integrations

### Prometheus & Alertmanager

Here are example alerting rules for your Prometheus or Alertmanager configuration.

<details>

```yaml
groups:
- name: VaultSecrets
  rules:
  - alert: VaultSecretExpiresSoon
    expr: vault_secret_expiry_days_remaining < 7
    for: 1h
    labels:
      severity: critical
    annotations:
      summary: "Vault secret is expiring soon: {{ $labels.path }}"
      description: "The secret at path '{{ $labels.path }}' will expire in {{ printf \"%.2f\" $value }} days. Please rotate it immediately. Owner: {{ $labels.owner_email }}. Usage: {{ $labels.usage_description }}"

  - alert: VaultSecretExpiresIn30Days
    expr: vault_secret_expiry_days_remaining < 30
    for: 24h
    labels:
      severity: warning
    annotations:
      summary: "Vault secret is expiring in less than 30 days: {{ $labels.path }}"
      description: "The secret at path '{{ $labels.path }}' will expire in {{ printf \"%.2f\" $value }} days. Please plan to rotate it. Owner: {{ $labels.owner_email }}. Usage: {{ $labels.usage_description }}"

  - alert: VaultSecretHasNoExpiryDate
    expr: vault_secret_has_no_expiry_date == 1
    for: 12h
    labels:
      severity: warning
    annotations:
      summary: "Vault secret is missing expiry date: {{ $labels.path }}"
      description: "The secret at path '{{ $labels.path }}' does not have the 'expiry_date' metadata field set. This violates the secrets management policy."
  ```

</details>


## Grafana Dashboard

You can import the following JSON model directly into Grafana to get a pre-built dashboard.

<details>

```json
{
  "__inputs": [],
  "__requires": [],
  "annotations": { "list": [ { "builtIn": 1, "datasource": { "type": "grafana", "uid": "-- Grafana --" }, "enable": true, "hide": true, "iconColor": "rgba(0, 211, 255, 1)", "name": "Annotations & Alerts", "type": "dashboard" } ] },
  "editable": true, "fiscalYearStartMonth": 0, "graphTooltip": 0, "id": null, "links": [],
  "panels": [
    { "gridPos": { "h": 9, "w": 18, "x": 0, "y": 0 }, "id": 2, "options": { "footer": { "fields": "", "reducer": ["sum"], "show": false }, "showHeader": true }, "pluginVersion": "10.1.1", "targets": [ { "datasource": { "type": "prometheus" }, "exemplar": false, "expr": "vault_secret_expiry_days_remaining", "format": "table", "instant": true, "interval": "", "legendFormat": "", "refId": "A" } ], "title": "All Vault Secrets", "transformations": [ { "id": "labelsToFields", "options": {} }, { "id": "organize", "options": { "excludeByName": {}, "indexByName": { "Time": 0, "owner_email": 2, "path": 1, "usage_description": 3, "Value": 4 }, "renameByName": { "Value": "Days Remaining", "owner_email": "Owner", "path": "Secret Path", "usage_description": "Usage Description" } } } ], "type": "table" },
    { "gridPos": { "h": 5, "w": 6, "x": 18, "y": 0 }, "id": 4, "options": { "colorMode": "value", "graphMode": "area", "justifyMode": "auto", "orientation": "auto", "reduceOptions": { "calcs": ["lastNotNull"], "fields": "", "values": false }, "textMode": "auto", "thresholds": { "mode": "absolute", "steps": [ { "color": "green", "value": null }, { "color": "orange", "value": 29 }, { "color": "red", "value": 7 } ] }, "unit": "d" }, "pluginVersion": "10.1.1", "targets": [ { "datasource": { "type": "prometheus" }, "exemplar": false, "expr": "min(vault_secret_expiry_days_remaining)", "interval": "", "legendFormat": "Soonest Expiry", "refId": "A" } ], "title": "Soonest Expiry", "type": "stat" },
    { "gridPos": { "h": 4, "w": 6, "x": 18, "y": 5 }, "id": 6, "options": { "colorMode": "value", "graphMode": "area", "justifyMode": "auto", "orientation": "auto", "reduceOptions": { "calcs": ["lastNotNull"], "fields": "", "values": false }, "textMode": "auto", "thresholds": { "mode": "absolute", "steps": [ { "color": "green", "value": null }, { "color": "red", "value": 0.5 } ] }, "unit": "none" }, "pluginVersion": "10.1.1", "targets": [ { "datasource": { "type": "prometheus" }, "exemplar": false, "expr": "sum(vault_secret_has_no_expiry_date)", "interval": "", "legendFormat": "", "refId": "A" } ], "title": "Secrets Missing Expiry Date", "type": "stat" }
  ],
  "refresh": "", "schemaVersion": 37, "style": "dark", "tags": [], "templating": { "list": [] }, "time": { "from": "now-6h", "to": "now" }, "timepicker": {}, "timezone": "", "title": "Vault Secret Expiry", "version": 1, "weekStart": ""
}

</details>

```

## License

This project is licensed under the MIT License.
