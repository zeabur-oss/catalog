//! The `utoipa` schemas describe the same values as the schemars ones.

#![cfg(all(feature = "utoipa", feature = "schema"))]

#[cfg(test)]
mod tests {
    #![expect(clippy::disallowed_types, reason = "a schema is JSON of any shape")]

    use schemars::schema_for;
    use serde_json::{Value, json};
    use utoipa::PartialSchema;
    use zeabur_catalog_format::component::InputValue;

    /// The type of each branch of an untagged enum's schema, in order.
    fn branch_types(schema: &Value, key: &str) -> Vec<Value> {
        schema[key]
            .as_array()
            .unwrap()
            .iter()
            .map(|branch| branch["type"].clone())
            .collect()
    }

    #[test]
    fn input_value_is_a_flag_a_whole_number_or_text() {
        let utoipa = serde_json::to_value(InputValue::schema()).unwrap();
        let schemars = schema_for!(InputValue).to_value();
        let want = vec![json!("boolean"), json!("integer"), json!("string")];
        assert_eq!(branch_types(&utoipa, "oneOf"), want);
        assert_eq!(branch_types(&schemars, "anyOf"), want);
    }
}
