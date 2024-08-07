# operator-exporter
This repository is part of a wider exporting architecture for the FinOps Cost and Usage Specification (FOCUS). This component is tasked with the creation of a generic exporting pipeline, according to the description given in a Custom Resource (CR). After the creation of the CR, the operator reads the "exporting" configuration part and creates three resources: a deployment with a generic prometheus exporter inside, a configmap containing the configuration and a service that exposes the prometheus metrics. The given endpoint is supposed to be a CSV file containing a FOCUS report. Then, it creates a new CR for another operator, which start a generic scraper that scrapes the data and uploads it to a database.

## Dependencies
To run this repository in your Kubernetes cluster, you need to have the following images in the same container registry:
 - prometheus-exporter-generic
 - prometheus-scraper-generic
 - operator-scraper
 - prometheus-resource-exporter-azure

There is also the need to have an active Databricks cluster, with SQL warehouse and notebooks configured. Its login details must be placed in the database-config CR.

## Configuration
To start the exporting process, see the "config-sample.yaml" file. It includes the database-config CR.
The deployment of the operator needs a secret for the repository, called `registry-credentials` in the namespace `finops`.

The exporter container is created in the namespace of the CR. The exporter container looks for a secret in the CR namespace called `registry-credentials-default`

## Installation
### Prerequisites
- go version v1.21.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### Installation with HELM
```sh
$ helm repo add krateo https://charts.krateo.io
$ helm repo update krateo
$ helm install finops-operator-exporter krateo/finops-operator-exporter
```

## Tested platforms
 - Azure


## Bearer-token for Azure
In order to invoke Azure API, the exporter needs to be authenticated first. In the current implementation, it utilizes the Azure REST API, which require the bearer-token for authentication. For each target Azure subscription, an application needs to be registered and assigned with the Cost Management Reader role.

Once that is completed, run the following command to obtain the bearer-token (1h validity):
```
curl -X POST -d 'grant_type=client_credentials&client_id=<CLIENT_ID>&client_secret=<CLIENT_SECRET>&resource=https%3A%2F%2Fmanagement.azure.com%2F' https://login.microsoftonline.com/<TENANT_ID>/oauth2/token
```