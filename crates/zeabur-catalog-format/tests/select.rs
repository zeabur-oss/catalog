//! Runs every case of `conformance/select/` against `select`.

#[cfg(test)]
mod tests {
    use std::fs;
    use std::path::Path;

    use zeabur_catalog_format::check;
    use zeabur_catalog_format::select::{self, SelectCase, Warning};

    fn sorted(warnings: Vec<Warning>) -> Vec<String> {
        let mut out: Vec<String> = warnings.into_iter().map(|w| format!("{w:?}")).collect();
        out.sort();
        out
    }

    #[test]
    fn every_select_case_passes() {
        let dir = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../conformance/select");
        let mut ran = Vec::new();
        for entry in fs::read_dir(&dir).unwrap() {
            let path = entry.unwrap().path();
            let case: SelectCase = check::read(&path).unwrap_or_else(|e| panic!("{e}"));
            let name = path.file_name().unwrap().to_string_lossy().into_owned();
            match (
                select::select(&case.components, &case.config),
                case.want.error,
            ) {
                (Err(got), Some(want)) => assert_eq!(got, want, "{name}"),
                (Ok(got), None) => {
                    assert_eq!(got.components, case.want.components, "{name}");
                    assert_eq!(got.left_out, case.want.left_out, "{name}");
                    assert_eq!(sorted(got.warnings), sorted(case.want.warnings), "{name}");
                }
                (got, want) => panic!("{name}: got {got:?}, want error {want:?}"),
            }
            ran.push(name);
        }
        assert!(!ran.is_empty(), "no cases in {}", dir.display());
    }
}
