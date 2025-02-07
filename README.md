# FinOps Operator Exporter
This repository is part of the wider exporting architecture for the Krateo Composable FinOps and manages the exporting from API endpoints of FOCUS cost reports. 

## Summary

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Examples](#examples)
4. [Configuration](#configuration)

## Overview
This finops-operator-exporter is tasked with the creation of a generic exporting pipeline, according to the description given in a Custom Resource (CR). After the creation of the CR, the operator reads the "exporting" configuration and creates three resources: a deployment with a generic prometheus exporter inside, a configmap containing the exporter configuration CR and a service that exposes the deployment. The given endpoint is supposed to be a CSV file containing a FOCUS report. Then, it creates a new CR for the FinOps Operator Scraper, which starts a generic scraper to upload the data to a database. The configmap is mounted as a volume inside the deployment and the service is used to expose the metrics collected by the exporter in the prometheus format.

## Architecture
![Krateo Composable FinOps Operator Exporter](resources/images/KCF-operator-exporter.png)

## Examples
```yaml
apiVersion: finops.krateo.io/v1
kind: DatabaseConfig
metadata:
  name: # DatabaseConfig name
  namespace: # DatabaseConfig namespace
spec:
  username: # username string
  passwordSecretRef: # object reference to secret with password
    name: # secret name
    namespace: # secret namespace
    key: # secret key
---
apiVersion: finops.krateo.io/v1
kind: ExporterScraperConfig
metadata:
  name: # ExporterScraperConfig name
  namespace: # ExporterScraperConfig namespace
spec:
  exporterConfig: # same as krateoplatformops/finops-prometheus-exporter-generic
    provider: 
      name: # name of the provider config
      namespace: # namespace of the provider config
    api: # the API to call
      path: # the path inside the domain
      method: GET # the method to call the API with
      endpointRef: # secret with the url in the format http(s)://host:port, it can contain variables, such as http://<varName>.com:<envExample>, which will be compiled with the additionalVariables fields
        name: 
        namespace:
    # metricType: # optional, one of: cost, resource; default value: cost
    pollingIntervalHours: # int
    additionalVariables:
      varName: sample
      # Variables whose value only contains uppercase letters are taken from environment variables
      # FROM_THE_ENVIRONMENT must be the name of an environment variable inside the target exporter container (e.g., kubernetes services)
      envExample: FROM_THE_ENVIRONMENT
  scraperConfig: # same fields as krateoplatformops/finops-prometheus-scraper-generic
    tableName: # tableName in the database to upload the data to
    # api: # api to the exporter, optional (if missing, it uses the exporter)
    pollingIntervalHours: # int
    scraperDatabaseConfigRef: # See above kind DatabaseConfig
      name: # name of the databaseConfigRef CR 
      namespace: # namespace of the databaseConfigRef CR
```
If the field `metricType` is set to `cost`, then the API in `url` must expose a FOCUS report in a CSV file. Otherwise, if set to `resource`, it must expose usage metrics according to the JSON/OPENAPI schema in the folder resources and the field `additionalVariables` must contain a field `ResourceId` with the identifier of the resources to be used in the database as external key to reference the cost metric from the usage metric (i.e., the same as the field `resourceId` of the focusConfig CR).

The field `spec.scraperConfig.api` can be left empty if the exporter and scraper are both configured. The operator will compile this field automatically.

The CR can be configured to include a `provider`, which is an object reference to a set of CRs that identify, for a given provider, which resources and which additional metrics should be exported and scraped. For example, for the CPU usage of virtual machines on Azure:
```yaml
apiVersion: finops.krateo.io/v1
kind: ProviderConfig
metadata:
  name: azure
  namespace: finops
spec:
  resourcesRef:
  - name: azure-virtual-machines
    namespace: finops
- - -
apiVersion: finops.krateo.io/v1
kind: ResourceConfig
metadata:
  name: azure-virtual-machines
  namespace: finops
spec:
  resourceFocusName: Virtual machine
  metricsRef:
  - name: azure-vm-cpu-usage
    namespace: finops
- - -
apiVersion: finops.krateo.io/v1
kind: MetricConfig
metadata:
  name: azure-vm-cpu-usage
  namespace: finops
spec:
  metricName: Percentage CPU
  endpoint:
    resourceSuffix: /providers/microsoft.insights/metrics?api-version=2023-10-01
  timespan: month
  interval: PT15M
```
For these metrics, the exporting/scraping pipeline is automatically started and the field `metricType` is automatically populated.

## Configuration
To start the exporting process, see the examples section. The configuration sample includes the database-config CR.

The exporter container is created in the namespace of the CR. The exporter container looks for a secret in the CR namespace called `registry-credentials`, configurable in the HELM chart.

The FOCUS data needs to be in the CSV format and the `Tags` column has to use the following format:
```
{"CostCenter": "1234","Cost department": "Marketing","env": "prod","org": "trey","Project": "Foo"}
```

### Prerequisites
- go version v1.21.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### Installation with HELM
The operator can be installed through its [Helm chart](https://github.com/krateoplatformops/finops-operator-exporter-chart).

### Dependencies
To run this repository in your Kubernetes cluster, you need to install the following Krateo Composable FinOps components
 - prometheus-exporter-generic
 - prometheus-scraper-generic
 - operator-scraper
 - prometheus-resource-exporter-azure
 - finops-database-handler

Additionally, you need to have a working CrateDB instance to store the scraped data.

### Bearer-token for Azure
In order to invoke Azure API, the exporter needs to be authenticated first. In the current implementation, it utilizes the Azure REST API, which require the bearer-token for authentication. For each target Azure subscription, an application needs to be registered and assigned with the Cost Management Reader role.

Once that is completed, run the following command to obtain the bearer-token (1h validity):
```
curl -X POST -d 'grant_type=client_credentials&client_id=<CLIENT_ID>&client_secret=<CLIENT_SECRET>&resource=https%3A%2F%2Fmanagement.azure.com%2F' https://login.microsoftonline.com/<TENANT_ID>/oauth2/token
```