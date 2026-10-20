# webhook-logger

A lightweight Go service that receives [Alertmanager webhook](https://prometheus.io/docs/alerting/latest/configuration/#webhook_config) notifications and prints each individual alert as one compact JSON line to STDOUT.

## Purpose

This repository provides a small bridge between Alertmanager notifications and a Kubernetes log-processing pipeline. Alertmanager can group multiple firing and resolved alerts into a single webhook request. The service splits that request into individual JSON log records so each alert can be inspected, transformed, and forwarded independently.

In the SCHIFF integration, the Logging Operator collects these records, maps them to the AC alert data model through a dedicated `ClusterFlow`, and forwards them to the CDI Kafka bus through a `ClusterOutput`.

```text
Alertmanager → webhook-logger → container STDOUT
                                  ↓
                         Logging Operator ClusterFlow
                         JSON parsing and AC field mapping
                                  ↓
                         ClusterOutput → CDI Kafka → consumer
```

The Go service handles webhook reception and JSON output. AC mapping, cluster/site resolution, Kafka connectivity, and Kafka credentials are configured separately in the deployment's GitOps repository.

## Output format

Each output record contains the fields of one `alerts[]` element, including its own `status`, `labels`, `annotations`, `startsAt`, `endsAt`, `generatorURL`, and `fingerprint`. Additional fields in an alert are preserved. Group-level webhook fields such as `commonLabels`, `groupKey`, and the overall notification status are not included.

For example, a webhook containing two alerts produces two separate lines:

```json
{"annotations":{"summary":"Worker pool is stuck"},"labels":{"alertname":"MachineDeploymentStuck","cluster_name":"example-1","namespace":"metal3-prd-ffm1"},"status":"firing"}
{"annotations":{"summary":"Worker pool recovered"},"labels":{"alertname":"MachineDeploymentStuck","cluster_name":"example-2","namespace":"metal3-ref-bln500"},"status":"resolved"}
```

Newlines inside string values are escaped, so each alert occupies exactly one log line. Operational messages, such as the startup message, are written to STDERR; a downstream flow should select valid alert records before forwarding them.

An empty `alerts` array produces no output. Malformed JSON, missing/null `alerts` arrays, or invalid alert objects return HTTP 400. HTTP 200 acknowledges writing the alert records to STDOUT; it does not confirm downstream Kafka delivery or consumer processing.

## Usage

Configure an Alertmanager receiver:

```yaml
receivers:
  - name: webhook-logger
    webhook_configs:
      - url: http://webhook-logger.webhook-logger:9095/webhook
        send_resolved: true
```

Then read the alerts:

```bash
kubectl logs -f deployment/webhook-logger -n webhook-logger
```

## Endpoints

| Path | Method | Description |
|------|--------|-------------|
| `/webhook` | POST | Receives Alertmanager payloads, prints one JSON line per alert to STDOUT |
| `/healthz` | GET | Liveness probe |

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `PORT` | `9095` | HTTP listen port |
| `WEBHOOK_TOKEN` | Unset | Optional Bearer token required for `/webhook` |
| `WEBHOOK_TOKEN_FILE` | Unset | Read the Bearer token from this file; takes precedence over `WEBHOOK_TOKEN`. Leading and trailing whitespace is removed. |

Without a configured token, `/webhook` accepts unauthenticated requests. When authentication is enabled, configure Alertmanager's webhook `http_config.authorization` to send the matching Bearer token. `/healthz` remains unauthenticated.

## Build

```bash
docker build -t webhook-logger .
```
