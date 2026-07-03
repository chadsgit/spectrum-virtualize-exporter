# TODO

## Collector gaps

### Extend existing collectors

- **`mdisk_collector.go`: add status gauge, enable by default** -- collector exists in `collector/`
  but is `defaultDisabled` and exposes `status` only as a label on a capacity metric, not as an
  alertable gauge. Add a dedicated `mdisk_status` gauge (0=online, 1=degraded, 2=offline) so
  Prometheus rules can alert on degraded or offline managed disks. Nick's check_ibm_flashsystem
  Nagios script treats degraded mdisks as WARN and offline as CRITICAL; the current collector
  produces no signal either way.

- **`nodecanister_collector.go`: expose cache size** -- `lsnodecanister` is already called but only
  extracts `status`. The `memory` field in the same response is the per-node cache size in GiB.
  Expose as `nodecanister_cache_size_bytes` with `node_name` label. Confirmed: ART1-SAN01
  Canister 1 = 128 GiB per the Storage Virtualize GUI.

- **`portfc_collector.go`: add FC port error counters and I/O stats** -- current collector calls
  `lsportfc` (status + attachment only). Add a second call to `lsportfcstats` per port to expose
  TX/RX bytes and BB credit zero counts. BB credit zero is the key fabric congestion signal;
  without it you cannot distinguish a slow drain device from a healthy port under load.

### New collectors

- **Per-volume IOPS and latency** (`lsvdiskstats`) -- `volume_collector.go` covers capacity/status
  only. `lsvdiskstats` returns per-volume read/write IOPS, throughput, and response time. This is
  the only way to identify which specific volume is driving array-level load (e.g. which Kroll
  PMS volume is responsible for the 38-40K IOPS seen on ART1-SAN01).

- **RC consistency group status** (`lsrcconsistgrp`) -- `rcopy_collector.go` covers individual
  relationships (`lsrcrelationship`) but not the consistency groups that contain them. A broken
  consistency group (e.g. "DR system not found") does not surface in any current metric.
  Expose group name, status, and state as labeled gauges. Same pattern as rcopy_collector.go.

- **FlashCopy map status** (`lsfcmap`) -- snapshot and clone map health. Expose map name,
  status, progress, and copy_rate. Alert on anything not `idle_or_copied` or `copying`.
  Pairs with `lsfcconsistgrp` for grouped snapshot sets.

- **Policy-based replication** (`lsreplicationpolicy` + `lsreplicationrelationship`) -- IBM
  Storage Virtualize introduced policy-based replication as the successor to legacy Metro/Global
  Mirror. Not collected at all. Expose policy name, relationship name, state, and sync status
  as labeled gauges. Discovered gap: a policy-based config was left broken on ART1-SAN01
  (pointing to ART1-DRSAN01, "not found") with no monitoring to catch it.

- **Quorum disk status** (`lsquorum`) -- silent failure mode. If quorum disks are lost, the
  system cannot survive a node failure without operator intervention. No current metric covers
  this. Expose quorum disk ID, status, and active state as labeled gauges.

- **License status** (`lslicense`) -- which Storage Virtualize features are licensed (FlashCopy,
  Metro Mirror, Global Mirror, compression, encryption). Useful for detecting unlicensed feature
  use and auditing compliance posture. Expose as info-style labeled gauge per feature.

- **IP partnership status** (`lspartnership`) -- partner system connectivity for Global Mirror
  WAN links. Expose partnership name, type (Metro/Global), location, and status. Required if
  any customer uses Global Mirror over IP; currently invisible.

- **Event log alert count** (`lseventlog`) -- Nick's check_ibm_flashsystem Nagios script scans
  `lseventlog -alert yes -message no -monitoring no -fixed no -expired no` for fault patterns:
  drive fault, temperature threshold exceeded, managed disk error count, SAS error counts, space
  warnings, path warnings, remote copy timeout, node memory failure, etc. The exporter has no
  equivalent. Expose `eventlog_unfixed_alert_count` as a gauge (count of active unfixed alerts).
  Optionally expose per-severity counts if the API response includes severity levels. This catches
  fault conditions that don't surface through any current per-component metric.

- **Enclosure slot fault LED** (`lsenclosureslot`) -- `enclosure_collector.go` covers the
  enclosure-level `lsenclosure` status but not per-slot fault LED state. Nick's script calls
  `lsenclosureslot` per enclosure to detect lit fault LEDs, which indicate a failed drive at that
  specific slot and can surface the fault before the drive registers as offline in `lsdrive`.
  Expose per-slot fault LED state as a binary gauge (0=off, 1=on) with enclosure_id and slot_id
  labels. Secondary state: identify light (slow_flashing) masks fault LED; expose separately.
