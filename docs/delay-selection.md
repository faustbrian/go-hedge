# Delay selection

A fixed delay repeats between scheduling slots. A schedule supplies one
positive successive delay for every hedge slot, not an absolute start offset.
Denied admission and continued construction failures also consume a slot and
advance the next timer, so ordinals are not guaranteed attempt starts. A dynamic
function receives only the hedge ordinal and the previous delay; percentile
storage remains caller-owned and bounded.

The Tail at Scale suggests targeting the long tail, commonly a current p95 or
p99, instead of hedging normal requests. Measure by resource and workload;
cold or mixed-rollout data can be misleading. Zero or negative dynamic delays
are terminal policy failures. The total timeout bounds schedules whose next
timer falls beyond the remaining operation lifetime.
