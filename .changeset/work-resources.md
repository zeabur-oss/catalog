---
default: minor
---

# Resources are amounts of each resource, and the index moves to version 3

`resources.work` on a component lists each kind of work it does. An entry gives `requests`, `limits` and `use` in Kubernetes quantities of `cpu` and `memory`, such as `use: {memory: 500Mi}`, and `use` replaces `resources.workMemoryMiB`. `index.json` publishes each as an object with `memoryMiB` and `cpuMillis`, so a resource added later is one more field in that object. `measured.<env>` moves to the same shape: `idle` replaces `idleMemoryMiB` and `idleCPUm`, and `requests` replaces `requestMemoryMiB`. An environment measured in the old format is left out of `index.json` until the next weekly run measures it again. `builds/zeabur-builder` publishes the requests and limits of a build pod, copied from `buildkitd.resources` of its chart. The controller creates that pod, and no render shows it. An installer adds the build pod's `requests.memoryMiB` to the measured `requests.memoryMiB` when it counts the memory a team needs. Zeabur Builder moves to chart 0.1.2-beta.1, which sets that value.
