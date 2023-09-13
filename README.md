# Webhook Notifier for Grafana Alerting

This is a simple implementation of a [webhook notifier](https://grafana.com/docs/grafana/latest/alerting/alerting-rules/manage-contact-points/webhook-notifier/) for Grafana alerting. When Grafana alerts, it sends a JSON payload to a webhook (if configured). This webhook will save them to an sqlite database (TODO). Used for testing Grafana alert notifications.

## Getting Started

1. Start the service by executing the binary: `./grafana-webhook`
2. Configure a [contact point](https://grafana.com/docs/grafana/latest/alerting/fundamentals/contact-points/) for a webhook in Grafana Alerting and set the `url` to http://localhost:4000

## License

Distributed under the MIT License. See [LICENSE](LICENSE) for more information.
