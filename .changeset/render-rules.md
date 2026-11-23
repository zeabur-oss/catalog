---
default: minor
---

# zeabur-catalog render checks pods, images and webhooks

`zeabur-catalog render` now runs the render rules that `catalog render` ran: every image has a digest unless `unpinnedImages` names it, a chart gives the same objects on two renders, and every pod meets the component's Pod Security level. The Pod Security checks come from `zeabur-catalog-engine`.

A component that serves an admission webhook must declare `health`, so that what needs the webhook installs only after it answers. `catalog render` is removed.
