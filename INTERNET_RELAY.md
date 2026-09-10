# Internet worker groups

v1.6 adds an optional internet transport for the existing image-job protocol.
The coordinator and workers make outbound HTTPS requests to a relay. This works
across ordinary NAT/router boundaries without forwarding a port on either laptop,
provided their networks allow the relay connection.

An operator must supply a publicly reachable HTTPS relay. No public relay service,
free GPU account or automatic internet-wide peer discovery is bundled. The same
ORIGIN0 executable can run the relay; no Python or model files are needed for it.

## Relay operator

Run a Linux build or the Windows executable on a machine with a reachable domain
and a TLS certificate. Set these environment variables before launching:

```text
ORIGIN0_RELAY_MODE=1
ORIGIN0_RELAY_ACCESS_KEY=<a randomly generated secret of at least 32 characters>
ORIGIN0_RELAY_LISTEN=0.0.0.0:443
ORIGIN0_RELAY_TLS_CERT=/path/to/certificate.pem
ORIGIN0_RELAY_TLS_KEY=/path/to/private-key.pem
```

Alternatively omit the TLS file variables and use the default
`127.0.0.1:8790` behind your HTTPS reverse proxy. Without TLS files, the relay
refuses to bind an unencrypted public address. Proxy `/origin-relay/` and
`/health`, allow at least 33 MB request/response bodies, and use upstream timeouts
above 45 seconds. External clients always configure the public **HTTPS** URL.

Share the registration key only with people allowed to create groups. The relay
keeps at most eight groups and four 24 MiB in-flight request reservations, with
additional bounded serialization/response overhead. It returns backpressure
errors when full. Groups expire after 20 minutes without an authenticated host
poll. Stop deletes the group. In-memory relay state is lost on relay restart;
create a new group/invitations after a restart.

## Coordinator

In **Worker PCs → Connect through an internet relay**, enter the HTTPS URL and
registration key, then choose **Start internet group**. Create one invitation for
each worker and deliver it privately. The registration key is not included in
worker invitations and is not saved by the app.

## Worker

Set up the image engine, paste the invitation into **Contribute this PC**, choose
a finite job allowance, and press **Join and contribute**. The existing Stop,
time/thread limits, progress, cancellation and validated PNG return path apply.
Both participants need compatible image model packs.

## Transport properties and limits

- HTTPS protects each device-to-relay connection. An additional AES-256-GCM layer
  encrypts the pool request/result between coordinator and invited workers.
  Direction, room and request identity are authenticated with each envelope.
- The relay receives no encryption key and cannot read image prompts, worker
  credentials or outputs. It still sees IP addresses, timing and envelope sizes.
- Worker-specific authentication, job leases, expiry, cancellation and PNG checks
  remain enforced by the coordinator inside the encrypted protocol.
- Relay registration keys, room host credentials and invitations are distinct.
  Only the group host can poll requests, deliver responses or delete a group.
- Requests allow only the existing four image-job operations. No arbitrary
  command, source-code execution or filesystem browsing is added.
- This is coordinator/relay job distribution. It does **not** partition diffusion
  weights, pool VRAM, train a model across peers, or provide torrent block transfer.
  Hivemind/Petals/libp2p integration remains a separate measured development step.
- These limits constrain a small private volunteer group. Large public deployments
  also need ordinary operational monitoring, rate limiting and bandwidth planning.

The automated acceptance test runs a real HTTPS relay and coordinator, encrypted
lease/heartbeat/result-error traffic, authentication failures and cancellation.
It does not replace a real deployment test between two different home networks.
