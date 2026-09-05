# Development ML stub

The stub implements the Go↔ML v1 HTTP contract without a model. It echoes the
request identity and returns one `observed` point for each usable NDVI input;
missing or unusable observations become `missing`. It never produces anomaly
events.

Run it with the development Compose profile and the already-built Go image:

```sh
MODEL_VERSION=dev-stub GO_IMAGE=ghcr.io/benzocloud/cosmohack-go:latest \
  docker compose -f deploy/compose.yaml --profile dev-ml-stub \
  up postgres migrate ml-stub go-stub
```

Set `ML_STUB_MODE=busy`, `ML_STUB_MODE=timeout`, or `ML_STUB_MODE=invalid` to
exercise the corresponding client error paths. The real `ml` service remains
unchanged and production still requires `ML_IMAGE`.
