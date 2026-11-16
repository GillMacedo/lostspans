# Examples

These files were written for this repository. The services, images and hosts are made up.

- `broken/checkout.yaml`: a ConfigMap, a Deployment and a CronJob with OpenTelemetry settings that lose telemetry, one mistake per rule shown in the README.
- `fixed/checkout.yaml`: the same objects with each mistake fixed. It must stay quiet.
- `compose/`: a Compose project whose service reads an `env_file` with mistakes in it.
- `github-actions/lostspans.yml`: a pull request workflow to copy into your repository. It builds lostspans, renders a Helm chart with `helm template`, and checks the rendered output and your Compose files. It fails on findings and on input lostspans can't trust.
