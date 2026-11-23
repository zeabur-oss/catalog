---
default: minor
---

# The check's problems are typed

`zeabur-catalog-format` has `problem::Problem`: the file, the place in it as a `FieldPath` (written `health[0].condition`) and a `ProblemKind`, which wraps the enum of the check that found it. `check::from_yaml` takes the file's path and returns these, and `ReadError::Check` holds them. A reader matches on the kind instead of the text.
