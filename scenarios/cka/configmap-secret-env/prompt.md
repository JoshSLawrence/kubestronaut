Deployment `api` in namespace `config` needs its configuration.

1. Create an immutable ConfigMap `app-config` in `config` from the file
   `/opt/config/app.properties` on `controlplane`. The key must be
   `app.properties`.
2. Create a Secret `db-creds` in `config` with `username=app` and
   `password=S3cure!pass`.
3. Update deployment `api` so that its container gets the environment
   variables `DB_USER` and `DB_PASSWORD` from the Secret keys `username` and
   `password`, and sees the ConfigMap as the read-only file
   `/etc/app/app.properties`.

The pods must be running with the new configuration when you are done.
