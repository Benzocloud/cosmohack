# Development ML stub

The stub implements the Go↔ML v1 HTTP contract without a model. It echoes the
request identity and returns one `observed` point for each usable NDVI input;
missing or unusable observations become `missing`. It never produces anomaly
events.

For local development, run the development Compose profile. It uses the
separate `go-stub` service only to route requests to `ml-stub`:

```sh
MODEL_VERSION=dev-stub GO_IMAGE=ghcr.io/benzocloud/cosmohack-go:latest \
  docker compose -f deploy/compose.yaml --profile dev-ml-stub \
  up postgres migrate ml-stub go-stub
```

Set `ML_STUB_MODE=busy`, `ML_STUB_MODE=timeout`, or `ML_STUB_MODE=invalid` to
exercise the corresponding client error paths. The real `ml` service remains
unchanged and production still requires `ML_IMAGE`.

When `DEPLOY_ENABLED=true` and the production marker is absent, the main branch
pipeline publishes this image and deploys the production `go` service with it
as `ML_IMAGE`. Once `backend/ml/PRODUCTION_READY` is committed, the real ML
image becomes the only automatic production choice. The pipeline continues to
publish this stub image so the previous release manifest can roll back to a
known compatible pair; selecting it for a new deployment remains explicit.
Both deploy paths require the `CDSE_CLIENT_ID` and
`CDSE_CLIENT_SECRET` repository secrets because the production Go composition
initializes the satellite source at startup.
