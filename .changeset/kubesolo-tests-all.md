---
default: patch
---

# The `kubesolo` profile, like `k3s` and `rke2`, tests every component that single-node supports and whose `distributions` include it

`distributions` now lists the distributions to test a component on, and `testedOn` the ones where that test passed.
