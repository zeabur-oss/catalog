//! Runs every case of `conformance/select/` against `select`.

#[cfg(test)]
mod tests {
    use std::collections::BTreeMap;
    use std::fs;
    use std::path::Path;

    use serde::Deserialize;
    use zeabur_catalog_format::select::{
        self, Candidate, Chosen, Config, LeftOut, SelectError, Warning,
    };

    #[derive(Deserialize)]
    #[serde(rename_all = "camelCase", deny_unknown_fields)]
    struct Case {
        #[expect(dead_code, reason = "the case's rule, for people")]
        description: String,
        components: Vec<Candidate>,
        config: Config,
        want: Want,
    }

    #[derive(Deserialize)]
    #[serde(rename_all = "camelCase", deny_unknown_fields)]
    struct Want {
        #[serde(default)]
        components: Vec<Chosen>,
        #[serde(default)]
        left_out: BTreeMap<String, LeftOut>,
        #[serde(default)]
        warnings: Vec<Warning>,
        error: Option<SelectError>,
    }

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
            let case: Case = serde_saphyr::from_str(&fs::read_to_string(&path).unwrap())
                .unwrap_or_else(|e| panic!("{}: {e}", path.display()));
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
