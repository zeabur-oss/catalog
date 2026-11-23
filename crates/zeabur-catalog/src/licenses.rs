//! `zeabur-catalog licenses-update`: `config/licenses.yaml` from the SPDX License List.

use serde::Deserialize;

/// The SPDX License List, `https://spdx.org/licenses/licenses.json`.
#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct LicenseList {
    license_list_version: ListVersion,
    licenses: Vec<License>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct License {
    license_id: LicenseId,
    is_osi_approved: bool,
}

/// A `licenseListVersion`: digits and dots, so it cannot break the YAML it is written into.
#[derive(Debug, Deserialize)]
#[serde(try_from = "String")]
struct ListVersion(String);

impl TryFrom<String> for ListVersion {
    type Error = String;

    fn try_from(version: String) -> Result<Self, Self::Error> {
        if version.is_empty() || !version.chars().all(|c| c.is_ascii_digit() || c == '.') {
            return Err(format!("licenseListVersion {version:?} is not a version"));
        }
        Ok(Self(version))
    }
}

/// An SPDX short identifier: `idstring = 1*(ALPHA / DIGIT / "-" / ".")`, SPDX 3.0.1, "SPDX
/// license expressions" (<https://spdx.github.io/spdx-spec/v3.0.1/annexes/spdx-license-expressions/>),
/// with the trailing `+` of deprecated ids such as `GPL-1.0+`. Any other character could break
/// the YAML it is written into.
#[derive(Debug, Deserialize)]
#[serde(try_from = "String")]
struct LicenseId(String);

impl TryFrom<String> for LicenseId {
    type Error = String;

    fn try_from(id: String) -> Result<Self, Self::Error> {
        let stem = id.strip_suffix('+').unwrap_or(&id);
        if stem.is_empty()
            || !stem
                .chars()
                .all(|c| c.is_ascii_alphanumeric() || c == '-' || c == '.')
        {
            return Err(format!("licenseId {id:?} is not an SPDX identifier"));
        }
        Ok(Self(id))
    }
}

/// `current`, the text of `config/licenses.yaml`, with `spdxVersion` and `osiApproved` from
/// `list`, the text of `licenses.json`. What follows the `osiApproved` list, such as
/// `optInOnly`, stays as it is.
pub(crate) fn update(current: &str, list: &str) -> Result<String, String> {
    let list: LicenseList =
        serde_json::from_str(list).map_err(|e| format!("licenses.json: {e}"))?;
    let ListVersion(version) = list.license_list_version;
    let (_, rest) = current
        .split_once("\nosiApproved:\n")
        .ok_or("config/licenses.yaml: no osiApproved")?;
    let kept: String = rest
        .split_inclusive('\n')
        .skip_while(|line| line.starts_with("  - "))
        .collect();
    let mut ids: Vec<String> = list
        .licenses
        .into_iter()
        .filter(|l| l.is_osi_approved)
        .map(|l| l.license_id.0)
        .collect();
    ids.sort_unstable();
    let mut out = format!(
        "# OSI-approved SPDX identifiers, generated from the SPDX License List {version}\n\
         # (https://spdx.org/licenses/licenses.json, isOsiApproved). Regenerate with:\n\
         # curl -sSL https://spdx.org/licenses/licenses.json | cargo run -p zeabur-catalog -- licenses-update\n\
         spdxVersion: \"{version}\"\n\
         osiApproved:\n"
    );
    for id in ids {
        out.push_str("  - ");
        out.push_str(&id);
        out.push('\n');
    }
    out.push_str(&kept);
    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::update;

    const LIST: &str = r#"{
        "licenseListVersion": "3.30",
        "licenses": [
            {"licenseId": "MIT", "isOsiApproved": true, "name": "MIT License"},
            {"licenseId": "BUSL-1.1", "isOsiApproved": false},
            {"licenseId": "0BSD", "isOsiApproved": true},
            {"licenseId": "Apache-2.0", "isOsiApproved": true}
        ]
    }"#;

    const CURRENT: &str = "# old header\nspdxVersion: \"3.29.0\"\nosiApproved:\n  - MIT\n\
                           # Allowed where no profile recommends.\noptInOnly:\n  - BUSL-1.1 # Dragonfly\n";

    #[test]
    fn writes_the_osi_approved_ids_in_order_and_keeps_opt_in_only() {
        let want = "# OSI-approved SPDX identifiers, generated from the SPDX License List 3.30\n\
                    # (https://spdx.org/licenses/licenses.json, isOsiApproved). Regenerate with:\n\
                    # curl -sSL https://spdx.org/licenses/licenses.json | cargo run -p zeabur-catalog -- licenses-update\n\
                    spdxVersion: \"3.30\"\n\
                    osiApproved:\n  - 0BSD\n  - Apache-2.0\n  - MIT\n\
                    # Allowed where no profile recommends.\noptInOnly:\n  - BUSL-1.1 # Dragonfly\n";
        assert_eq!(update(CURRENT, LIST).unwrap(), want);
    }

    #[test]
    fn running_it_twice_changes_nothing() {
        let once = update(CURRENT, LIST).unwrap();
        assert_eq!(update(&once, LIST).unwrap(), once);
    }

    #[test]
    fn refuses_a_version_that_would_break_the_yaml() {
        let list = LIST.replace("3.30", "3\\\"");
        assert!(
            update(CURRENT, &list)
                .unwrap_err()
                .contains("is not a version")
        );
    }

    #[test]
    fn refuses_an_id_that_would_break_the_yaml() {
        let list = LIST.replace("\"MIT\"", "\"MIT\\n  - evil\"");
        assert!(
            update(CURRENT, &list)
                .unwrap_err()
                .contains("is not an SPDX identifier")
        );
    }
}
