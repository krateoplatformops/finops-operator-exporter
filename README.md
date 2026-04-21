# finops-operator-exporter

A Kubernetes operator that creates generic exporting pipelines for FOCUS cost reports from API endpoints, provisioning Prometheus exporters, configmaps, and services as part of the Krateo Composable FinOps architecture.

📖 **Full documentation**: [docs.krateo.io — finops-operator-exporter](https://docs.krateo.io/key-concepts/kcf/finops-components/finops-operator-exporter)

---

## Key features

- Automatically provisions a Prometheus exporter deployment, configmap, and service from a single Custom Resource
- Supports FOCUS cost reports in CSV/JSON format, resource usage metrics, and generic arbitrary data exports
- Integrates with the FinOps Operator Scraper to upload collected data to a CrateDB database

## Requirements

| Dependency | Minimum version |
|------------|----------------|
| Kubernetes | v1.31 |
| Krateo | v3.0.0 |
| operator-scraper | v0.5.0 |
| finops-database-handler | v0.5.3 |
| CrateDB | v5.9.6 |

## Install

```bash
helm repo add krateo https://charts.krateo.io
helm repo update
helm install finops-operator-exporter krateo/finops-operator-exporter --namespace krateo-system --create-namespace
```

> For advanced installation options, custom values, and upgrade instructions, see the [installation guide](https://docs.krateo.io/key-concepts/kcf/finops-components/finops-operator-exporter).

## Environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `POLLING_INTERVAL` | No | `300` | Polling interval of the operator in seconds |
| `MAX_RECONCILE_RATE` | No | `1` | Number of workers for the operator |
| `REGISTRY` | No | `ghcr.io/krateoplatformops` | Registry to pull the exporter image from |
| `REGISTRY_CREDENTIALS` | No | `registry-credentials` | Name of the secret holding registry credentials |
| `EXPORTER_VERSION` | No | `0.5.0` | Version of the exporter image |
| `EXPORTER_NAME` | No | `finops-prometheus-exporter` | Name of the exporter image |