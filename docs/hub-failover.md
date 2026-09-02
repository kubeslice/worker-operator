# Following a controller failover (worker-operator #467)

When the KubeSlice controller runs Active/Standby across two hub clusters, a worker has to notice
that leadership moved and reconnect to the hub that now holds it. This describes how the worker
does that, and what an operator has to provide for it to work.

Off by default. A worker with a single hub configured behaves exactly as it always has — no
resolution, no extra connections, no change.

## How it decides

Each hub publishes `status.activeController` on this worker's `Cluster` CR while it holds
leadership, and a Standby's mirrored copy repeats the Active's declaration. The worker polls both
pre-provisioned hub endpoints and applies one rule:

1. An unreachable hub has no say.
2. A hub that published nothing has no say. This is what a non-HA hub looks like.
3. A declaration naming an endpoint that is not one of the two configured hubs is **rejected**. The
   field chooses between endpoints you provisioned; it cannot point the worker somewhere else.
4. If the hubs agree, that is the answer. Agreement is the normal case — the Standby mirrors the
   Active's declaration, so both name the same hub.
5. If they disagree, the fresher declaration wins. This happens when a recovered old Active still
   names itself. It keeps the worker's behaviour single-valued; it does not resolve split brain,
   which the controller design lists as a non-goal.
6. If nothing usable comes back, the worker changes nothing and keeps its current connection.

A change has to hold across consecutive polls before the worker acts, so a single blip cannot move
it. When it does act, the worker shuts down cleanly and the kubelet restarts it; startup resolution
then picks the new hub. Gateways and tunnels run in their own pods, so the data plane is not
affected by that restart.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `HUB_SECONDARY_HOST_ENDPOINT` | *(unset)* | The other hub's API server. **Unset disables everything here.** |
| `HUB_SECONDARY_TOKEN_FILE` | `/var/run/secrets/kubernetes.io/hub-secondary-serviceaccount/token` | Credential for that hub |
| `HUB_SECONDARY_CA_FILE` | `/var/run/secrets/kubernetes.io/hub-secondary-serviceaccount/ca.crt` | CA for that hub |
| `HUB_RESOLVE_INTERVAL` | `10s` | How often to re-check |
| `HUB_RESOLVE_TIMEOUT` | `5s` | Bound on each read of a hub |
| `HUB_SWITCH_CONFIRMATIONS` | `2` | Consecutive agreeing polls before reconnecting |

`config/manager/manager.yaml` carries the env, volume mount and volume for the second credential,
commented out.

Two metrics are exported: `kubeslice_worker_hub_switches_total` and
`kubeslice_worker_hub_probe_errors_total{hub="primary|secondary"}`. The second is the one to alert
on — a hub that has been quietly unreachable for days is a problem you want to hear about before a
failover, not during one.

## The second credential

The worker authenticates to a hub with a token that hub minted. A token from hub A is not valid on
hub B, so the worker needs a **second** credential, for the Standby, mounted **before** the Active
fails — afterwards there is nothing left to hand it one.

**This step has no owner in either repository.** It belongs to the cluster registration flow and
the Helm charts, not to worker-operator or kubeslice-controller. Until it is part of registration,
install it by hand. The steps below are what the Kind demo uses.

The controller side already does its half: the Standby mirrors the worker's `ServiceAccount` and an
empty token `Secret` shell, and its own token controller fills that shell with a token valid on the
Standby. So the credential already exists on the Standby — it just has to be copied to the worker.

Set these to match your deployment:

```bash
export STANDBY=kind-hub-standby
export WORKER=kind-worker-1
export PROJECT_NS=kubeslice-avesha
export WORKER_SA=kubeslice-worker-worker-1
export WORKER_NS=kubeslice-system
```

Read the Standby-minted token and CA:

```bash
kubectl --context $STANDBY get secret $WORKER_SA \
  -n $PROJECT_NS -o jsonpath='{.data.token}' | base64 -d > /tmp/hub-b-token
```

```bash
kubectl --context $STANDBY get secret $WORKER_SA \
  -n $PROJECT_NS -o jsonpath='{.data.ca\.crt}' | base64 -d > /tmp/hub-b-ca.crt
```

Confirm it actually authenticates on the Standby before installing it. A token that is present but
invalid is worse than none, because it fails only at failover:

```bash
kubectl --server=$(kubectl --context $STANDBY config view -o \
  jsonpath='{.clusters[?(@.name=="'${STANDBY#kind-}'")].cluster.server}') \
  --certificate-authority=/tmp/hub-b-ca.crt \
  --token="$(cat /tmp/hub-b-token)" auth whoami
```

Install it on the worker:

```bash
kubectl --context $WORKER create secret generic hub-secondary-tenant-token \
  -n $WORKER_NS \
  --from-file=token=/tmp/hub-b-token \
  --from-file=ca.crt=/tmp/hub-b-ca.crt
```

Then uncomment the `HUB_SECONDARY_*` env, the volume mount and the volume in the worker's
deployment, set `HUB_SECONDARY_HOST_ENDPOINT` to the Standby's API server address, and restart the
worker.

```bash
rm -f /tmp/hub-b-token /tmp/hub-b-ca.crt
```

## Checking it works

With both hubs up, the worker logs which hub it resolved at startup and then stays quiet. Stop the
Active controller; once the Standby promotes itself, the worker logs `active hub changed`,
increments `kubeslice_worker_hub_switches_total`, exits, and comes back connected to the promoted
hub. Gateway pods should not restart at any point — that is the part worth watching.

## Connection observability (#469)

The worker's own `Cluster` CR on the hub carries two Conditions:

| Condition | Meaning |
|---|---|
| `ControllerConnected` | Is this worker currently reconciling against a hub |
| `ControllerEndpointSynced` | Does the connected endpoint match the currently resolved active hub |

Reasons: `Connected` (steady state), `ReconnectedAfterFailover` (the first reconcile after
following a resolved switch), `EndpointNotConfigured` (no secondary hub configured — the normal
non-HA case, status `Unknown`, not an error), `Reconnecting` (best-effort, written just before the
process restarts to follow a switch), `DialFailed`/`CertVerificationFailed`.

`DialFailed` and `CertVerificationFailed` are real reason values — see
`pkg/hub/hubclient.ClassifyConnectionError`, unit-tested against fake dial errors — but by
construction they will rarely persist on the CR: writing either one needs the same connection that
just failed. Metrics and logs remain the reliable live-detection channel:

- `kubeslice_worker_controller_reconnect_attempts_total{result}` — startup connection decisions
  (`primary`, `resolved-switch`, `unresolved`), next to the existing `kubeslice_worker_hub_switches_total`
  and `kubeslice_worker_hub_probe_errors_total` in `pkg/hub/failover/failover.go`.
- `kubeslice_worker_controller_last_sync_time_seconds` — when the last startup decision was made.

Four Events, all on the worker's `Cluster` CR: `ControllerEndpointChanged` (fires once, the same
reconcile that reports `ReconnectedAfterFailover`), `ControllerConnected` (fires on the transition
into `Connected`), `ControllerConnectionLost` (best-effort, written from the `Watch` callback right
before the process restarts to follow a switch), and `CertVerificationFailed` (defined, not
currently fired from any live path — same limitation as the condition reason above).
