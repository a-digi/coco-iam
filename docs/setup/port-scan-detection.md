# Port-scan detection setup

coco-iam can detect port scanning against this host — traffic aimed at ports the app isn't
listening on at all, which is otherwise invisible to it (a single HTTP process bound to one port
has no way to see packets addressed elsewhere). It does this by reading a kernel firewall log line
your OS's own `iptables` already knows how to produce — coco-iam never does the detecting itself,
only the ingesting.

This is documentation-only: coco-iam never modifies your host's firewall rules for this feature.
The one command below is something **you** run, once, when you're ready. See
`plan/port-scan-detection/plan.md` Phase B for the full design.

---

## 1. Add the logging rule

Append a single rate-limited `LOG` rule to the end of your `INPUT` chain:

```sh
sudo iptables -A INPUT -m limit --limit 30/min --limit-burst 10 \
  -j LOG --log-prefix "coco-portscan: " --log-level 4
```

What this does, precisely:

- **Appended (`-A`), not inserted** — it only fires for packets that reach the *end* of `INPUT`
  without having matched an earlier `ACCEPT`/`DROP`/`REJECT` rule. That's exactly the signature of
  a probe against a port nothing on this host handles.
- **Logs, does not drop.** Your existing firewall policy (whatever accepts/rejects traffic today)
  is completely unaffected — this rule only *observes*.
- **Rate-limited** (`--limit 30/min --limit-burst 10`) so a real, sustained scan can't turn this
  into a way to flood your own kernel log — a self-inflicted denial-of-service vector otherwise.
- **`--log-prefix "coco-portscan: "`** must match exactly — coco-iam's ingestion only parses lines
  containing this exact string, so it can't misfire on unrelated kernel log noise. If you need a
  different prefix for your own reasons, you'll need to also change
  `scanwatch.DefaultLogPrefix` in `api/src/security/scanwatch/parse.go` and rebuild.

This rule does not survive a reboot on its own — persist it the way you already persist the rest
of your iptables rules (`iptables-persistent` on Debian/Ubuntu, `/etc/iptables/rules-save` +
an init script on Alpine, etc.). That's an existing operational concern on your host, not something
specific to this feature.

---

## 2. Where the log lines end up

coco-iam auto-detects which of these is available, in this order — no configuration needed:

| | Ubuntu | Alpine |
|---|---|---|
| Init/log system | systemd + journald | OpenRC, no journald |
| Where the `LOG` line lands | the journal | `/var/log/messages` (busybox syslogd), by default |
| Read access needed | membership in the `systemd-journal` group, or root | read permission on the syslog file |

If neither is reachable, the admin Security page reports port-scan detection as unavailable and
says why — nothing fails silently.

---

## 3. Verify the rule is in place

```sh
sudo iptables -L INPUT -n --line-numbers | grep coco-portscan
```

You should see one line ending in `LOG flags 0 level 4 prefix "coco-portscan: "`.

To generate a real test hit (from another machine or a container — scanning `localhost` may not
traverse `INPUT` the same way depending on your setup):

```sh
nmap -sT -p 1-100 <this-host's-IP>
```

Then check that the log actually captured it:

```sh
# Ubuntu (journald)
journalctl -k --since "1 minute ago" | grep coco-portscan

# Alpine (syslog file)
grep coco-portscan /var/log/messages
```

If a matching line appears, coco-iam will pick it up on its next read and, once the same source IP
has touched enough distinct ports within the aggregation window (5 distinct ports within 5 minutes
by default — see `scanwatch.DefaultThreshold` / `scanwatch.DefaultWindow`), a scan episode will
appear under **Security → Port scans** in the admin console.
