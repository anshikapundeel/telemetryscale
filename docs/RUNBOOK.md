# Runbook

## SLOBurnRate: ingest-availability (page)
1. Open the TelemetryScale dashboard: queue depth, dropped samples/s, p99 batch latency.
2. Queue depth rising and retries rising: the backend is slow or down. Check `docker compose ps` and ClickHouse/Mimir logs.
3. Drops rising with a healthy backend: producers exceed capacity. Raise workers or batch size, or enable DropNewest.
4. Run `make triage` for a drafted summary. Treat it as a lead, not a conclusion.
5. If a disruptive restart is needed: `go run ./remediation/cmd/start -service <svc> -disruptive` then approve via signal.
6. After recovery, write a postmortem from `docs/postmortems/TEMPLATE.md` within 5 working days.

## SLOBurnRate: ingest-latency-250ms
1. Check ClickHouse part count and merges: `SELECT table, count() FROM system.parts WHERE active GROUP BY table`.
2. Too many small parts means batches are too small. Increase `-batch`.

## FleetGuard check failing
| Check | First look |
|---|---|
| tls | `openssl s_client -connect host:443 -servername host` and compare notAfter |
| dns | `dig @<server> <name>`; compare latency vs other resolvers |
| kerberos | `klist`, `kinit`; check clock skew with the KDC (over 5 minutes breaks auth) |
| ldap | bind with `ldapwhoami -x -H ldap://host`; check result code (49 = bad credentials) |
| dhcp | lease file utilisation vs pool size; look for abandoned leases |
| puppet | `puppet agent -t --noop`; check last_run timestamp and failed resources |
| haproxy | stats CSV: which server is DOWN and why (`check_status`) |
| smtp | `swaks --to x --server host` |

## Cardinality alert
`go run ./cmd/cardwatch -tenant <t>` lists labels above threshold. Fix at the source (drop or bucket the label), then confirm the series count falls.
