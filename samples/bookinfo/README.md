# Bookinfo (pure Go)

A re-implementation of the [Bookinfo sample](https://github.com/istio/istio/tree/master/samples/bookinfo) in **pure Go**, using
only the standard library — no external Go dependencies, no databases, no
language runtimes. All data is hard-coded or kept in memory.

The four services expose the same endpoints, ports, environment variables, and
behavior as the original sample, so all of the Istio traffic management demos
(routing, fault injection, mirroring, rate limiting, circuit breaking,
egress policies, ...) work unchanged with the manifests in
`samples/bookinfo/networking`.

| Service     | Original            | Go implementation                          | Port |
|-------------|---------------------|--------------------------------------------|------|
| `productpage` | Python / Flask     | `cmd/productpage` (templates + static assets embedded via `go:embed`) | 9080 |
| `details`   | Ruby / WEBrick     | `cmd/details`                              | 9080 |
| `reviews`   | Java / WebSphere Liberty | `cmd/reviews`                        | 9080 |
| `ratings`   | Node.js + MongoDB/MySQL | `cmd/ratings` (seeded data hard-coded) | 9080 |

The `mongodb` and `mysql` deployments are **not** needed: `ratings` v2 serves
the same seeded values (`Reviewer1: 5`, `Reviewer2: 4`) from memory.

## Differences from the original

- **No databases.** `ratings` v2 reads its "database" from hard-coded values
  matching the original seed scripts.
- **Sessions.** `productpage` uses a plain (unsigned) cookie instead of
  Flask's `itsdangerous`-signed session — fine for a demo.
- **`/metrics`.** Emits the same `request_result_total` counter as the
  original in Prometheus text format (hand-rolled, no
  `prometheus/client_golang`; the `_created` series is omitted).
- **gRPC.** The original `reviews` also served gRPC on 9081 for old
  productpage versions; the current sample only uses HTTP, so no gRPC here.
- Images are built `FROM scratch` (static binary + CA certificates).

## Behavior selected by environment variables

Same knobs as the original images:

| Variable                      | Service     | Effect                                                        |
|-------------------------------|-------------|---------------------------------------------------------------|
| `SERVICE_VERSION`             | ratings     | `v1` (default), `v2`, `v-faulty`, `v-delayed`, `v-unavailable`, `v-unhealthy` |
| `SERVICE_VERSION`             | details     | informational                                                 |
| `ENABLE_EXTERNAL_BOOK_SERVICE`| details     | `true`: fetch from the Google Books API (egress demo)         |
| `DO_NOT_ENCRYPT`              | details     | `true`: call the external API over HTTP (port 80)             |
| `ENABLE_RATINGS`              | reviews     | `true`: decorate reviews with star ratings (v2/v3)            |
| `STAR_COLOR`                  | reviews     | `black` (v2, 10s ratings timeout) or `red` (v3, 2.5s timeout) |
| `FLOOD_FACTOR`                | productpage | `>0`: fire N extra review requests per page view (rate limit demo) |
| `SERVICES_DOMAIN`             | all         | optional DNS suffix for cross-cluster setups                  |
| `DETAILS_HOSTNAME` / `DETAILS_SERVICE_PORT`, `REVIEWS_HOSTNAME` / `REVIEWS_SERVICE_PORT`, `RATINGS_HOSTNAME` / `RATINGS_SERVICE_PORT` | productpage, reviews | override upstream addresses |

## Endpoints

- `productpage`: `GET /` (alias `GET /index.html`), `GET /productpage`,
  `GET /health`, `POST /login`,
  `GET /logout`, `GET /metrics`, `GET /api/v1/products`,
  `GET /api/v1/products/{id}`, `GET /api/v1/products/{id}/reviews`,
  `GET /api/v1/products/{id}/ratings`, `GET /static/...`
- `details`: `GET /health`, `GET /details/{id}`
- `reviews`: `GET /health`, `GET /reviews/{id}`
- `ratings`: `GET /health`, `GET /ratings/{id}`, `POST /ratings/{id}`

## Build the container images

```sh
cd samples/bookinfo
make                                  # podman, all images, tag latest
make reviews-v3                       # build a single image
make CLI=docker HUB=ghcr.io/me TAGS=1.0   # custom CLI/registry/tag
make PUSH=1 HUB=ghcr.io/me            # build and push
```

This produces one image per version, mirroring the original sample's image
layout:

- `examples-bookinfo-go-productpage-v1`
- `examples-bookinfo-go-details-v1`, `examples-bookinfo-go-details-v2`
- `examples-bookinfo-go-reviews-v1`, `examples-bookinfo-go-reviews-v2`,
  `examples-bookinfo-go-reviews-v3`
- `examples-bookinfo-go-ratings-v1`, `examples-bookinfo-go-ratings-v2`

Each image is a single static binary plus the CA certificate bundle. The
per-version behavior (star color, external book API, "database-backed"
ratings) is baked in as `ENV` defaults — all eight images share the same
four binaries. Deployments can still override them at deploy time, e.g.
`kubectl set env deployment/ratings SERVICE_VERSION=v-faulty` for the fault
injection demo.

## Deploy to a cluster

The services are drop-in replacements (same names, ports, endpoints and
environment variables). The deployment manifest at
`samples/bookinfo/platform/kube/bookinfo.yaml` already references the published
Go images. From the repository root:

1. Build and push the images (above), if you are using a custom registry.
2. Apply the sample:

   ```sh
   kubectl apply -f samples/bookinfo/platform/kube/bookinfo.yaml
   ```

3. `reviews-v2` (black stars) and `reviews-v3` (red stars) are deployed with
   `bookinfo.yaml`. Enable flooding with
   `kubectl set env deployment/productpage FLOOD_FACTOR=100`.

4. Clean up with `kubectl delete -f samples/bookinfo/platform/kube/bookinfo.yaml`.
