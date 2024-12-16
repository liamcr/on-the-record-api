# On The Record API

API to interact with the Postgres backend for On The Record.

## Starting Up

Run the following command to start up the API.

```
go run ./cmd
```

This requires environment variables `DB_USER` and `DB_PASSWORD` to be set.

## Deployment

Build a new image that will be pushed to docker hub:

`docker buildx build --platform linux/amd64 -t liamcrocketdev/otr-api:VERSION .`

Where `VERSION` is a valid semver value (e.g. `0.3.0`)

Then push that image:

`docker push liamcrocketdev/otr-api:VERSION`

In GCP, navigate to Cloud Run, click `otr-api`, click `EDIT & DEPLOY NEW REVISION`, update the
version in the `Container image URL` field to match the version that was just pushed, and
click `DEPLOY`. The new version of the site should come up shortly after.
