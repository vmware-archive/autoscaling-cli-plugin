# Autoscaling CLI Plugin
The plugin can be used to enable the autoscaling service for an app from the CLI.

### Installation
```bash
cf install-plugin https://github.com/pivotal-cf-experimental/autoscaling-cli-plugin
```

### Usage
Example: If I had an app called "fibcpu" with a bound Autoscaler called "scaler," and wanted to create a scaling rule that scales the app up by one instance every five minutes when the CPU utilization is over 75% up to a max of 55 instances and down again when the CPU utilization is less than 50% one could run:
```bash
cf configure-autoscaling --min-threshold 50 --max-threshold 75 --max-instances 55 --min-instances 3 fib-cpu scaler
```
