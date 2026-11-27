//! The DNS name rules that Kubernetes object names and hostnames follow [k8s-names].
//!
//! [k8s-names]: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/

/// The longest DNS label.
const LABEL_MAX: usize = 63;

/// The longest DNS subdomain or hostname.
const SUBDOMAIN_MAX: usize = 253;

const fn is_label_byte(b: u8) -> bool {
    b.is_ascii_lowercase() || b.is_ascii_digit() || b == b'-'
}

/// Whether `name` is an RFC 1123 label: at most 63 lower-case letters, digits and `-`, starting
/// and ending with a letter or digit [k8s-label].
///
/// [k8s-label]: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
pub fn is_dns_label(name: &str) -> bool {
    let bytes = name.as_bytes();
    name.len() <= LABEL_MAX
        && bytes.iter().copied().all(is_label_byte)
        && bytes.first().is_some_and(u8::is_ascii_alphanumeric)
        && bytes.last().is_some_and(u8::is_ascii_alphanumeric)
}

/// Whether `name` is a lower-case hostname: at most 253 characters, every part between dots an
/// RFC 1123 label [rfc1123].
///
/// [rfc1123]: https://www.rfc-editor.org/rfc/rfc1123#section-2.1
pub fn is_hostname(name: &str) -> bool {
    name.len() <= SUBDOMAIN_MAX && name.split('.').all(is_dns_label)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn names_follow_the_dns_rules() {
        let label_63 = "a".repeat(63);
        let label_64 = "a".repeat(64);
        let dotted_253 = format!("{label_63}.{label_63}.{label_63}.{}", "a".repeat(61));
        let dotted_254 = format!("{dotted_253}a");
        let long_part = format!("{label_64}.example.com");
        // name, label, hostname
        let cases: &[(&str, bool, bool)] = &[
            ("web", true, true),
            ("1abc", true, true),
            (&label_63, true, true),
            (&label_64, false, false),
            ("", false, false),
            ("-web", false, false),
            ("web-", false, false),
            ("Web", false, false),
            ("we_b", false, false),
            ("shop.example.com", false, true),
            ("shop..com", false, false),
            (".shop", false, false),
            (&dotted_253, false, true),
            (&dotted_254, false, false),
            (&long_part, false, false),
        ];
        for &(name, label, hostname) in cases {
            assert_eq!(is_dns_label(name), label, "label {name:?}");
            assert_eq!(is_hostname(name), hostname, "hostname {name:?}");
        }
    }
}
