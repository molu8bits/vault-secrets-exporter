package exporter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// Exporter implements the prometheus.Collector interface
type Exporter struct {
	client      *api.Client
	kvMountPath string

	// Metric descriptions
	secretExpiryDays *prometheus.Desc
	secretNoExpiry   *prometheus.Desc
	scrapeErrors     prometheus.Counter
}

// NewExporter creates a new exporter
func NewExporter(client *api.Client, kvMountPath string) *Exporter {
	return &Exporter{
		client:      client,
		kvMountPath: kvMountPath,
		secretExpiryDays: prometheus.NewDesc(
			"vault_secret_expiry_days_remaining",
			"Number of days remaining until the secret expires. A negative value means the secret has expired.",
			[]string{"path", "owner_email", "usage_description"},
			nil,
		),
		secretNoExpiry: prometheus.NewDesc(
			"vault_secret_has_no_expiry_date",
			"Indicates if a secret does not have an expiry_date set (1 = no date, 0 = date exists).",
			[]string{"path"},
			nil,
		),
		scrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "vault_exporter_scrape_errors_total",
			Help: "Total number of errors encountered while scraping metrics from Vault.",
		}),
	}
}

// Describe implements prometheus.Collector
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.secretExpiryDays
	ch <- e.secretNoExpiry
	e.scrapeErrors.Describe(ch)
}

// Collect implements prometheus.Collector
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	paths, err := e.listAllSecrets(context.Background(), "")
	if err != nil {
		log.WithError(err).Error("Failed to list secrets")
		e.scrapeErrors.Inc()
		return
	}

	for _, path := range paths {
		metadata, err := e.client.KVv2(e.kvMountPath).GetMetadata(context.Background(), path)
		if err != nil {
			log.WithError(err).Warnf("Failed to get metadata for path: %s", path)
			e.scrapeErrors.Inc()
			continue
		}

		if metadata == nil || metadata.CustomMetadata == nil {
			log.Warnf("No custom metadata found for path: %s", path)
			ch <- prometheus.MustNewConstMetric(e.secretNoExpiry, prometheus.GaugeValue, 1, path)
			continue
		}

		expiryDateVal, ok := metadata.CustomMetadata["expiry_date"]
		if !ok {
			log.Warnf("No 'expiry_date' in metadata for path: %s", path)
			ch <- prometheus.MustNewConstMetric(e.secretNoExpiry, prometheus.GaugeValue, 1, path)
			continue
		}

		//expiryDateStr, ok := metadata.CustomMetadata["expiry_date"]
		expiryDateStr, ok := expiryDateVal.(string)
		if !ok || expiryDateStr == "" {
			log.Warnf("No 'expiry_date' in metadata for path: %s", path)
			ch <- prometheus.MustNewConstMetric(e.secretNoExpiry, prometheus.GaugeValue, 1, path)
			continue
		}

		// Signal that the secret DOES have an expiry date
		ch <- prometheus.MustNewConstMetric(e.secretNoExpiry, prometheus.GaugeValue, 0, path)

		expiryDate, err := time.Parse(time.RFC3339, expiryDateStr)
		if err != nil {
			log.WithError(err).Warnf("Invalid date format for path %s: %s", path, expiryDateStr)
			e.scrapeErrors.Inc()
			continue
		}

		daysRemaining := time.Until(expiryDate).Hours() / 24

		var ownerEmail string
		if ownerEmailVal, ok := metadata.CustomMetadata["owner_email"]; ok {
			if emailStr, ok := ownerEmailVal.(string); ok {
				ownerEmail = emailStr
			}
		}
		var usageDesc string
		if usageDescVal, ok := metadata.CustomMetadata["usage_description"]; ok {
			if descStr, ok := usageDescVal.(string); ok {
				usageDesc = descStr
			}
		}

		// ownerEmail, _ := metadata.CustomMetadata["owner_email"]
		// usageDesc, _ := metadata.CustomMetadata["usage_description"]

		ch <- prometheus.MustNewConstMetric(
			e.secretExpiryDays,
			prometheus.GaugeValue,
			daysRemaining,
			path,
			ownerEmail,
			usageDesc,
		)
	}
	e.scrapeErrors.Collect(ch)
}

// listAllSecrets recursively lists all secrets under a given path
func (e *Exporter) listAllSecrets(ctx context.Context, path string) ([]string, error) {
	var allPaths []string
	secret, err := e.client.Logical().List(fmt.Sprintf("%s/metadata/%s", e.kvMountPath, path))
	if err != nil {
		return nil, err
	}
	if secret == nil || secret.Data["keys"] == nil {
		return allPaths, nil
	}

	keys, ok := secret.Data["keys"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid key format at path: %s", path)
	}

	for _, k := range keys {
		keyStr := k.(string)
		newPath := strings.Trim(fmt.Sprintf("%s/%s", path, keyStr), "/")
		if strings.HasSuffix(keyStr, "/") {
			// This is a directory, recurse deeper
			subPaths, err := e.listAllSecrets(ctx, newPath)
			if err != nil {
				return nil, err
			}
			allPaths = append(allPaths, subPaths...)
		} else {
			// This is a secret
			allPaths = append(allPaths, newPath)
		}
	}
	return allPaths, nil
}
