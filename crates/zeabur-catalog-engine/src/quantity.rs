//! The canonical text of a resource quantity.
//!
//! It is the text
//! [`resource.Quantity`](https://pkg.go.dev/k8s.io/apimachinery/pkg/api/resource#Quantity) of
//! apimachinery v0.37.2 writes: `0.5` becomes `500m` and `1024Mi` becomes `1Gi`, and a value
//! already in canonical form keeps the text it was written with.

/// How a quantity is written: its suffix picks the format.
#[derive(Clone, Copy, PartialEq, Eq)]
enum Format {
    /// `e3`, `E-2`.
    DecimalExponent,
    /// `Ki` … `Ei`.
    BinarySi,
    /// `n` … `E`, or no suffix.
    DecimalSi,
}

const DECIMAL: [(&str, i32); 10] = [
    ("n", -9),
    ("u", -6),
    ("m", -3),
    ("", 0),
    ("k", 3),
    ("M", 6),
    ("G", 9),
    ("T", 12),
    ("P", 15),
    ("E", 18),
];
const BINARY: [(&str, u32); 6] = [
    ("Ki", 10),
    ("Mi", 20),
    ("Gi", 30),
    ("Ti", 40),
    ("Pi", 50),
    ("Ei", 60),
];

/// The canonical text of `text`, or `None` when it is not a quantity or is too large for this
/// port (Go caps those, which no chart needs).
#[must_use]
pub fn canonical(text: &str) -> Option<String> {
    if text == "0" {
        return Some(text.to_owned());
    }
    let parsed = Parsed::new(text)?;
    if parsed.keeps_text() {
        return Some(text.to_owned());
    }
    parsed.amount()?.canonical(parsed.format)
}

struct Parsed<'a> {
    negative: bool,
    num: &'a str,
    denom: &'a str,
    format: Format,
    /// The power of 10, or of 2 for [`Format::BinarySi`], of the suffix.
    exponent: i32,
}

impl<'a> Parsed<'a> {
    fn new(text: &'a str) -> Option<Self> {
        if text.is_empty() {
            return None;
        }
        let negative = text.starts_with('-');
        let rest = text.strip_prefix(['-', '+']).unwrap_or(text);
        let (num, rest) = split_digits(rest);
        let num = if num.is_empty() { "0" } else { num };
        let (denom, suffix) = rest.strip_prefix('.').map_or(("", rest), split_digits);
        let (format, exponent) = interpret(suffix)?;
        Some(Self {
            negative,
            num,
            denom,
            format,
            exponent,
        })
    }

    /// Whether Go keeps the text as written: it fits Go's fast path and is already canonical.
    fn keeps_text(&self) -> bool {
        let digits = self.num.len() + self.denom.len();
        match self.format {
            Format::BinarySi => {
                self.denom.is_empty()
                    && usize::try_from(self.exponent).is_ok_and(|e| digits + e * 3 / 10 < 15)
                    && self.num.parse::<i64>().is_ok_and(|value| {
                        value.checked_mul(1 << self.exponent).is_some() && value & 7 != 0
                    })
            }
            Format::DecimalExponent | Format::DecimalSi => {
                let shifted = [self.num, self.denom].concat();
                digits <= 18
                    && i64::try_from(self.denom.len()).is_ok_and(|denom| {
                        let scale = i64::from(self.exponent) - denom;
                        scale >= -9 && scale % 3 == 0
                    })
                    && !shifted.ends_with("000")
                    && !shifted.starts_with('0')
            }
        }
    }

    fn amount(&self) -> Option<Amount> {
        let digits: i128 = [self.num, self.denom].concat().parse().ok()?;
        let mut amount = Amount {
            mantissa: if self.negative { -digits } else { digits },
            exponent: -i32::try_from(self.denom.len()).ok()?,
        };
        match self.format {
            Format::BinarySi => {
                let shift = u32::try_from(self.exponent).ok()?;
                amount.mantissa = amount.mantissa.checked_mul(1_i128.checked_shl(shift)?)?;
            }
            Format::DecimalExponent | Format::DecimalSi => {
                amount.exponent = amount.exponent.checked_add(self.exponent)?;
            }
        }
        Some(amount.round_up_to_nano())
    }
}

fn split_digits(text: &str) -> (&str, &str) {
    text.split_at(
        text.find(|c: char| !c.is_ascii_digit())
            .unwrap_or(text.len()),
    )
}

fn interpret(suffix: &str) -> Option<(Format, i32)> {
    if let Some((_, exponent)) = DECIMAL.iter().find(|(s, _)| *s == suffix) {
        return Some((Format::DecimalSi, *exponent));
    }
    if let Some((_, exponent)) = BINARY.iter().find(|(s, _)| *s == suffix) {
        return Some((Format::BinarySi, i32::try_from(*exponent).ok()?));
    }
    let exponent = suffix.strip_prefix(['e', 'E'])?;
    let digits = exponent.strip_prefix(['+', '-']).unwrap_or(exponent);
    if digits.is_empty() || !digits.bytes().all(|b| b.is_ascii_digit()) {
        return None;
    }
    Some((Format::DecimalExponent, exponent.parse().ok()?))
}

/// `mantissa × 10^exponent`.
struct Amount {
    mantissa: i128,
    exponent: i32,
}

impl Amount {
    /// Go rounds a value below a nano up, away from zero, so a request for some resource gets
    /// some.
    fn round_up_to_nano(self) -> Self {
        if self.exponent >= -9 {
            return self;
        }
        let Some(divisor) = u32::try_from(-9 - self.exponent)
            .ok()
            .and_then(|power| 10_i128.checked_pow(power))
        else {
            return Self {
                mantissa: self.mantissa.signum(),
                exponent: -9,
            };
        };
        let rounded = self.mantissa / divisor + (self.mantissa % divisor).signum();
        Self {
            mantissa: rounded,
            exponent: -9,
        }
    }

    fn integer(&self) -> Option<i128> {
        if self.exponent >= 0 {
            return self
                .mantissa
                .checked_mul(10_i128.checked_pow(u32::try_from(self.exponent).ok()?)?);
        }
        let divisor = 10_i128.checked_pow(u32::try_from(-self.exponent).ok()?)?;
        (self.mantissa % divisor == 0).then_some(self.mantissa / divisor)
    }

    fn canonical(self, format: Format) -> Option<String> {
        if self.mantissa == 0 {
            return Some("0".to_owned());
        }
        if format == Format::BinarySi
            && let Some(value) = self.integer().filter(|value| value.abs() >= 1024)
        {
            let mut value = value;
            let mut power = 0;
            while value % 1024 == 0 && power < 60 {
                value /= 1024;
                power += 10;
            }
            let suffix = BINARY
                .iter()
                .find(|(_, p)| *p == power)
                .map_or("", |(s, _)| *s);
            return Some(format!("{value}{suffix}"));
        }
        let (mut mantissa, mut exponent) = (self.mantissa, self.exponent);
        while mantissa % 10 == 0 {
            mantissa /= 10;
            exponent += 1;
        }
        while exponent.rem_euclid(3) != 0 {
            mantissa = mantissa.checked_mul(10)?;
            exponent -= 1;
        }
        let suffix = match format {
            Format::DecimalExponent if exponent == 0 => String::new(),
            Format::DecimalExponent => format!("e{exponent}"),
            Format::BinarySi | Format::DecimalSi => {
                DECIMAL.iter().find(|(_, e)| *e == exponent)?.0.to_owned()
            }
        };
        Some(format!("{mantissa}{suffix}"))
    }
}

#[cfg(test)]
mod tests {
    use super::canonical;

    /// The output of `resource.MustParse(in).String()` with apimachinery v0.37.2.
    #[test]
    fn writes_quantities_as_go_does() {
        for (text, want) in [
            ("0", "0"),
            ("0.5", "500m"),
            ("1", "1"),
            ("1000m", "1"),
            ("100m", "100m"),
            ("1Gi", "1Gi"),
            ("104Mi", "104Mi"),
            ("1024Mi", "1Gi"),
            ("1536Mi", "1536Mi"),
            ("1023Ki", "1023Ki"),
            ("1024", "1024"),
            ("1500000000", "1500M"),
            ("1.5Gi", "1536Mi"),
            ("0.5Gi", "512Mi"),
            ("2k", "2k"),
            ("2000", "2k"),
            ("1e3", "1e3"),
            ("1E3", "1E3"),
            ("12e2", "1200"),
            ("1.5", "1500m"),
            ("0.0001", "100u"),
            ("100u", "100u"),
            ("1n", "1n"),
            ("0.5m", "500u"),
            ("-1", "-1"),
            ("-500m", "-500m"),
            ("5Gi", "5Gi"),
            ("3G", "3G"),
            ("1000k", "1M"),
            ("1000000", "1M"),
            ("0.1Ki", "102400m"),
            ("10e-3", "10e-3"),
            ("1Ei", "1Ei"),
            ("1Mi", "1Mi"),
            ("512", "512"),
            ("+1", "+1"),
            (".5", "500m"),
            ("5.", "5."),
            ("2Ti", "2Ti"),
        ] {
            assert_eq!(canonical(text).as_deref(), Some(want), "{text}");
        }
        for text in ["abc", "1.2.3", ""] {
            assert_eq!(canonical(text), None, "{text}");
        }
    }
}
