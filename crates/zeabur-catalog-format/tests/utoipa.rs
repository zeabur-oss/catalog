//! The `utoipa` schemas describe the same values as the schemars ones.

#![cfg(all(feature = "utoipa", feature = "schema"))]

#[cfg(test)]
mod tests {
    #![expect(clippy::disallowed_types, reason = "a schema is JSON of any shape")]

    use schemars::schema_for;
    use serde_json::{Value, json};
    use utoipa::{PartialSchema, ToSchema};
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
    fn input_value_is_a_flag_a_whole_number_text_or_a_map() {
        let utoipa = serde_json::to_value(InputValue::schema()).unwrap();
        let schemars = schema_for!(InputValue).to_value();
        let want = vec![
            json!("boolean"),
            json!("integer"),
            json!("string"),
            json!("object"),
        ];
        assert_eq!(branch_types(&utoipa, "oneOf"), want);
        assert_eq!(branch_types(&schemars, "anyOf"), want);
    }

    /// A map holds input values, so the schema refers to itself. Collecting the
    /// schemas an OpenAPI document needs stops at that reference.
    #[test]
    fn collecting_the_schemas_of_a_map_ends() {
        let mut schemas = Vec::new();
        InputValue::schemas(&mut schemas);
        let utoipa = serde_json::to_value(InputValue::schema()).unwrap();
        assert_eq!(
            utoipa["oneOf"][3]["additionalProperties"]["$ref"],
            json!("#/components/schemas/InputValue")
        );
    }
}
