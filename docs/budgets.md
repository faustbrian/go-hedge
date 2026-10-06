# Budgets and amplification

`MaxHedges` bounds one logical operation. A shared `Budget` bounds additional
work across concurrent logical operations. Every budget declares a finite
positive `Capacity` bounded by `MaxBudgetCapacity`. `OutstandingBudget(B)`
admits at most `B` simultaneous or completed-but-unconsumed hedges across every
policy sharing the instance and releases a permit when the coordinator consumes
or reclaims the result.

Budget denial is an observable admission decision, not a downstream error. The
existing attempts continue; their deterministic selected result is returned.
For independent resources, create one finite shared budget instance per known
resource. The built-in budget intentionally does not create an unbounded map
from attacker-controlled resource strings.

Retry and hedge layers must draw from one hard amplification budget. Separate
limits of `R` retries and `H` hedges can otherwise create up to
`(R + 1) * (H + 1)` executions.

## Shared resilience budget

New compositions should set `Config.UseResilienceBudget` and leave `Budget`
nil. `Do` then requires exactly one scope from the published
`github.com/faustbrian/go-resilience/v2` or retained
`github.com/faustbrian/go-resilience` v1 API attached to its context.
Attaching both versions, in either order, is a local `ErrInvalidPolicy` failure
before factory invocation or admission through either owner. Without an attached
scope, the existing v1 `ErrBudgetScopeRequired` identity is retained.
It reuses an outer physical attempt when present, admits each hedge with that
attempt as parent, and completes the returned permit when the hedge result is
consumed or reclaimed. Selection stays with the attached version for the whole
execution; a borrowed permit remains the outer caller's completion obligation.

The v2 route classifies only direct v2 budget rejections as capacity denial and
only direct standard cancellation sentinels as cancellation. It does not inspect
wrapped application errors through the v1 classifier. The explicit v1 route
retains its existing error traversal and admission behavior.

`Budget` and `OutstandingBudget` remain as a standalone compatibility path.
Configuring both budget owners is rejected. Capacity exhaustion increments
`Report.BudgetDenied` and leaves admitted attempts running; missing scope,
cancellation, closed scope, or invalid lineage remains a typed local failure
and cannot be classified as a downstream result.
