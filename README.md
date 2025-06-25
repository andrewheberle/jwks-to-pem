# jwks-to-pem

This program will retrieve the contents of a JSON Web Key Set (JWKS) and write the public keys in PEM format to a specified output directory.

If changes are detected in the keys then a reload of another service can be triggered by either sending a signal to a defined process or sending a HTTP request to a URL.

## Motivation

This was written to allow HAProxy to verify a JWT when the keys are published via a JWKS, so HAProxy could be configured similar to the following:

```haproxy
http-request set-var(txn.bearer) http_auth_bearer
http-request set-var(txn.jwt_alg) var(txn.bearer),jwt_header_query('$.alg')

acl jwt_alg_allowed var(txn.jwt_alg) -m str "RS256"
acl jwt_k1_ok var(txn.bearer),jwt_verify(txn.jwt_alg,"/path/to/keys/k1.pem") 1
acl jwt_k2_ok var(txn.bearer),jwt_verify(txn.jwt_alg,"/path/to/keys/k2.pem") 1

http-request deny unless jwt_alg_allowed
http-request deny if !jwt_k1_ok !jwt_k2_ok
```

In this case `jwks-to-pem` can be run periodically as follows:

```sh
./jwks-to-pem --url "https://example.com/path/to/jwks.json" \
    --pattern "k{{ .Index }}.pem" \
    --out "/path/to/keys" \
    --reload.pidfile "/path/to/haproxy.pid" \
    --reload.signal "SIGUSR2"
```

If changes to the keys are made, HAProxy is then reloaded to pick up the changes.

The command also supports a "cron" option (see below) to run on a schedule.

## Command Line Options

| Option           | Description                         | Default/Notes                      |
|------------------|-------------------------------------|------------------------------------|
| --debug          | Enable additional logging           | false                              |
| -o, --out        | Output directory for keys           | No default (prints keys to stdout) |
| -p, --pattern    | Go template naming pattern for keys | {{ .KeyID }}.pem                   |
| --reload.method  | HTTP method for reloads             | POST                               |
| --reload.pid     | PID to signal for reloads           |                                    |
| --reload.pidfile | File to lookup PID for reloads from |                                    |
| --reload.signal  | Signal for process based reloads    | SIGHUP                             |
| --reload.url     | URL for HTTP based reloads          |                                    |
| --timeout        | Timeout to retreive JWKS            | 5s                                 |
| -u, --url        | URL of JWKS                         | No default (required)              |

The options `--reload.pid` and `--reload.pidfile` are mutually exclusive, along with `--reload.url` and `--reload.pid`/`--reload.pidfile`.

All of the above options may be provided as environment variables prefixed by `JWKS_`, for example setting the following enviroment variables is equivalent to the command line used above:

```sh
JWKS_URL="https://example.com/path/to/jwks.json"
JWKS_RELOAD_PIDFILE="/path/to/haproxy.pid"
JWKS_RELOAD_SIGNAL="SIGUSR2"
JWKS_PATTERN="k{{ .Index }}.pem"
JWKS_OUT="/path/to/keys"
```

### Cron Mode

The "cron" sub-command may be used to have the process run as a daemon that triggers checks based on the provided `--schedule` which is schedule in crontab syntax as per the example below:

```sh
# run weekly at 1:15am
jwks-to-pem <other options> cron --schedule "15 1 * * 0"
```

The crontab schedule may be provided via the `JWKS_CRON_SCHEDULE` environment variable.

## Reloads

If one of the `--reload.pid`,  `--reload.pidfile` or `--reload.url` options are provided a reload will be triggered when changed to the downloaded keys are detected.

In then case of `--reload.pid` or `--reload.pidfile` the signal defined by `--reload.signal` will be sent.

If `--reload.url` was provided a HTTP request using the method set by `--reload.method` is performed.

## Docker

A docker container is published and can be used as follows:

```sh
docker run -v /path/to/keys:/keys ghcr.io/andrewheberle/jwks-to-pem \
    --url "https://example.com/path/to/jwks.json" \
    --out "/keys" \
    --pattern "k{{ .Index }}.pem"
```

Using PID based reloads will not work when running as a container unless the container you wish to trigger a reload in and this container share the same PID namespace.

Based on the initial example with HAProxy and having both run as containers, this could be achieved as follows:

```sh
# Start HAproxy
docker run --name haproxy --detach \
    -v /path/to/haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro \
    -v /path/to/keys:/keys:ro \
    haproxy:lts

# Periodically refresh keys
docker run --rm --pid container:haproxy \
    -v /path/to/keys:/keys \
    -e JWKS_OUT="/keys" \
    -e JWKS_URL="https://example.com/path/to/jwks.json" \
    -e JWKS_PATTERN="k{{ .Index }}.pem" \
    -e JWKS_RELOAD_PID="1" \
    -e JWKS_RELOAD_SIGNAL="SIGUSR2" \
    --user 99 \
    ghcr.io/andrewheberle/jwks-to-pem
```

The `--user 99` value above is to match the UID of the `haproxy` user in the `haproxy:lts` image so the reload works. 

For services that allow reloads via a HTTP POST/GET, issues around permissions and PID namespaces are not a consideration, so an example may be:

```sh
docker run --rm \
    -v /path/to/keys:/keys \
    -e JWKS_OUT="/keys" \
    -e JWKS_URL="https://example.com/path/to/jwks.json" \
    -e JWKS_PATTERN="k{{ .Index }}.pem" \
    -e JWKS_RELOAD_URL="http://otherservice:9090/-/reload" \
    ghcr.io/andrewheberle/jwks-to-pem
```
