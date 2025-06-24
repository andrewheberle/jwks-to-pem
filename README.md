# jwks-to-pem

This program will retrieve the contents of a JSON Web Key Set (JWKS) and write the public keys in PEM format to a specified output directory.

If changes are detected in the keys then a reload of another service can be triggered by either sending a signal to a defined process or sending a HTTP request to a URL.

## Motivation

This was written to allow HAProxy to verify a JWT when the keys are published via a JWKS, so HAProxy could be defined configured similar to the following:

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
