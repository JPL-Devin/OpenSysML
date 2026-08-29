use std::collections::HashMap;

use serde_json::Value;

use crate::scenario::Expect;
use opensysml::Status;

pub fn status_matches(expected: Option<&str>, actual: Status) -> bool {
    Status::from_canonical_name(expected.unwrap_or("OK")) == Some(actual)
}

pub fn compare(expect: &Expect, actual: &Value) -> Vec<String> {
    let mut failures = Vec::new();
    if let Some(response) = &expect.response {
        compare_value(response, actual, "$", &mut failures);
    }
    for path in &expect.non_empty {
        match lookup(actual, path) {
            Some(value) if !is_default(value) => {}
            Some(_) => failures.push(format!("{path} is empty")),
            None => failures.push(format!("{path} is absent")),
        }
    }
    for path in &expect.absent {
        if let Some(value) = lookup(actual, path) {
            if !is_default(value) {
                failures.push(format!("{path} is not absent/default: {value}"));
            }
        }
    }
    for (path, needle) in &expect.contains {
        match lookup(actual, path) {
            Some(Value::String(value)) if value.contains(needle) => {}
            Some(value) => failures.push(format!("{path}={value} does not contain {needle:?}")),
            None => failures.push(format!("{path} is absent")),
        }
    }
    for (path, needles) in &expect.contains_all {
        if path.split('.').any(|segment| segment == "*") {
            let values = values_at(actual, path);
            if values.is_empty() {
                failures.push(format!("{path} is absent or not text/list"));
            } else {
                for needle in needles {
                    if !values
                        .iter()
                        .any(|value| value.as_str().is_some_and(|text| text == needle))
                    {
                        failures.push(format!("{path} does not contain member {needle:?}"));
                    }
                }
            }
            continue;
        }
        match lookup(actual, path) {
            Some(Value::String(value)) => {
                for needle in needles {
                    if !value.contains(needle) {
                        failures.push(format!("{path} does not contain {needle:?}"));
                    }
                }
            }
            Some(Value::Array(values)) => {
                for needle in needles {
                    if !values.iter().any(|value| value == needle) {
                        failures.push(format!("{path} does not contain member {needle:?}"));
                    }
                }
            }
            Some(_) | None => {
                let values = values_at(actual, path);
                if values.is_empty() {
                    failures.push(format!("{path} is absent or not text/list"));
                } else {
                    for needle in needles {
                        if !values
                            .iter()
                            .any(|value| value.as_str().is_some_and(|text| text == needle))
                        {
                            failures.push(format!("{path} does not contain member {needle:?}"));
                        }
                    }
                }
            }
        }
    }
    for (path, wanted) in &expect.counts {
        check_count(path, *wanted, false, actual, &mut failures);
    }
    for (path, wanted) in &expect.min_counts {
        check_count(path, *wanted, true, actual, &mut failures);
    }
    failures
}

fn check_count(
    path: &str,
    wanted: usize,
    minimum: bool,
    actual: &Value,
    failures: &mut Vec<String>,
) {
    let Some(value) = lookup(actual, path) else {
        failures.push(format!("{path} is absent"));
        return;
    };
    let count = match value {
        Value::Array(items) => items.len(),
        Value::Object(items) => items.len(),
        _ => {
            failures.push(format!("{path} is not a list/map"));
            return;
        }
    };
    if (minimum && count < wanted) || (!minimum && count != wanted) {
        let relation = if minimum { "at least" } else { "exactly" };
        failures.push(format!(
            "{path} has {count} entries, want {relation} {wanted}"
        ));
    }
}

fn compare_value(expected: &Value, actual: &Value, path: &str, failures: &mut Vec<String>) {
    match (expected, actual) {
        (Value::Object(wanted), Value::Object(got)) => {
            for (key, value) in wanted {
                let Some(actual) = got.get(key) else {
                    failures.push(format!("{path}.{key} is absent"));
                    continue;
                };
                compare_value(value, actual, &format!("{path}.{key}"), failures);
            }
        }
        (Value::Array(wanted), Value::Array(got)) => {
            if wanted.len() != got.len() {
                failures.push(format!(
                    "{path} has {} entries, want {}",
                    got.len(),
                    wanted.len()
                ));
                return;
            }
            for (index, (wanted, got)) in wanted.iter().zip(got).enumerate() {
                compare_value(wanted, got, &format!("{path}.{index}"), failures);
            }
        }
        (Value::Number(wanted), Value::Number(got)) => {
            if !numbers_equal(wanted, got) {
                failures.push(format!("{path}: got {got}, want {wanted}"));
            }
        }
        (wanted, got) if wanted != got => {
            failures.push(format!("{path}: got {got}, want {wanted}"));
        }
        _ => {}
    }
}

fn numbers_equal(expected: &serde_json::Number, actual: &serde_json::Number) -> bool {
    if let (Some(expected), Some(actual)) = (expected.as_i64(), actual.as_i64()) {
        return expected == actual;
    }
    if let (Some(expected), Some(actual)) = (expected.as_u64(), actual.as_u64()) {
        return expected == actual;
    }
    match (expected.as_f64(), actual.as_f64()) {
        (Some(expected), Some(actual)) => {
            if expected == actual {
                true
            } else {
                let scale = expected.abs().max(actual.abs());
                scale != 0.0 && (expected - actual).abs() / scale <= 1e-9
            }
        }
        _ => false,
    }
}

fn is_default(value: &Value) -> bool {
    match value {
        Value::Null => true,
        Value::Bool(false) => true,
        Value::String(value) if value.is_empty() => true,
        Value::Number(value) => {
            value.as_i64() == Some(0) || value.as_u64() == Some(0) || value.as_f64() == Some(0.0)
        }
        Value::Array(value) => value.is_empty(),
        Value::Object(value) => value.is_empty(),
        _ => false,
    }
}

pub fn lookup<'a>(value: &'a Value, path: &str) -> Option<&'a Value> {
    lookup_parts(value, &path.split('.').collect::<Vec<_>>())
}

fn values_at<'a>(value: &'a Value, path: &str) -> Vec<&'a Value> {
    let parts = path.split('.').collect::<Vec<_>>();
    values_parts(value, &parts)
}

fn values_parts<'a>(value: &'a Value, parts: &[&str]) -> Vec<&'a Value> {
    let Some((head, rest)) = parts.split_first() else {
        return vec![value];
    };
    if *head == "*" {
        match value {
            Value::Array(items) => items
                .iter()
                .flat_map(|item| values_parts(item, rest))
                .collect(),
            Value::Object(items) => items
                .values()
                .flat_map(|item| values_parts(item, rest))
                .collect(),
            _ => Vec::new(),
        }
    } else {
        match value {
            Value::Object(items) => items
                .get(*head)
                .map_or_else(Vec::new, |item| values_parts(item, rest)),
            Value::Array(items) => head
                .parse::<usize>()
                .ok()
                .and_then(|index| items.get(index))
                .map_or_else(Vec::new, |item| values_parts(item, rest)),
            _ => Vec::new(),
        }
    }
}

fn lookup_parts<'a>(value: &'a Value, parts: &[&str]) -> Option<&'a Value> {
    let Some((head, rest)) = parts.split_first() else {
        return Some(value);
    };
    if *head == "*" {
        match value {
            Value::Array(values) => values.iter().find_map(|value| lookup_parts(value, rest)),
            Value::Object(values) => values.values().find_map(|value| lookup_parts(value, rest)),
            _ => None,
        }
    } else {
        let next = match value {
            Value::Object(values) => values.get(*head),
            Value::Array(values) => head
                .parse::<usize>()
                .ok()
                .and_then(|index| values.get(index)),
            _ => None,
        }?;
        lookup_parts(next, rest)
    }
}

pub fn label_instance_ids(value: &mut Value) {
    let mut labels = HashMap::new();
    label_ids(value, &mut labels);
}

fn label_ids(value: &mut Value, labels: &mut HashMap<i64, String>) {
    match value {
        Value::Object(object) => {
            let is_instance = object.contains_key("type_symbol_id")
                && object.contains_key("feature_values")
                && object.contains_key("id");
            if is_instance {
                if let Some(child) = object.get_mut("id") {
                    label_id(child, labels);
                }
            }
            if let Some(child) = object.get_mut("instance_id") {
                label_id(child, labels);
            }
            for (key, child) in object.iter_mut() {
                if !(is_instance && key == "id") && key != "instance_id" {
                    label_ids(child, labels);
                }
            }
        }
        Value::Array(items) => {
            for item in items {
                label_ids(item, labels);
            }
        }
        _ => {}
    }
}

fn label_id(value: &mut Value, labels: &mut HashMap<i64, String>) {
    if let Some(id) = value.as_i64() {
        let next = format!("@{}", labels.len() + 1);
        let label = labels.entry(id).or_insert(next).clone();
        *value = Value::String(label);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::scenario::Expect;
    use serde_json::json;

    #[test]
    fn relative_real_tolerance_has_both_boundaries() {
        let expect: Expect = serde_json::from_value(json!({"response": 100.0})).unwrap();
        assert!(compare(&expect, &json!(100.0 + 1e-10)).is_empty());
        assert!(!compare(&expect, &json!(100.0 + 1e-6)).is_empty());
    }

    #[test]
    fn named_lists_require_exact_length() {
        let expect: Expect = serde_json::from_value(json!({"response": [1, 2]})).unwrap();
        assert!(!compare(&expect, &json!([1, 2, 3])).is_empty());
        assert!(compare(&expect, &json!([1, 2])).is_empty());
    }

    #[test]
    fn defaults_and_wildcards_are_observable() {
        let actual = json!({"error": "", "elements": [{"id": 1}, {"id": 2}]});
        let expect: Expect = serde_json::from_value(json!({
            "response": {"error": ""},
            "absent": ["missing"],
            "non_empty": ["elements"],
            "contains_all": {"elements.*.id": ["1"]}
        }))
        .unwrap();
        assert!(!compare(&expect, &actual).is_empty());
        let actual = json!({"error": "", "elements": [{"id": "1"}, {"id": "2"}]});
        assert!(compare(&expect, &actual).is_empty());
        let expect_both: Expect = serde_json::from_value(json!({
            "contains_all": {"elements.*.id": ["1", "2"]}
        }))
        .unwrap();
        assert!(compare(&expect_both, &actual).is_empty());
        assert!(!compare(
            &expect_both,
            &json!({"elements": [{"id": "1"}, {"id": "3"}]})
        )
        .is_empty());
        assert_eq!(lookup(&actual, "elements.1.id"), Some(&json!("2")));
    }

    #[test]
    fn instance_ids_are_labelled_consistently() {
        let mut actual = json!({
            "instance": {"id": 9, "type_symbol_id": "T", "feature_values": {}},
            "instances": [{"id": 9, "type_symbol_id": "T", "feature_values": {}},
                          {"id": 12, "type_symbol_id": "T", "feature_values": {}}],
            "value": {"instance_id": 12}
        });
        label_instance_ids(&mut actual);
        assert_eq!(actual["instance"]["id"], "@1");
        assert_eq!(actual["instances"][0]["id"], "@1");
        assert_eq!(actual["instances"][1]["id"], "@2");
        assert_eq!(actual["value"]["instance_id"], "@2");
    }

    #[test]
    fn integer_precision_is_not_float_precision() {
        let expect: Expect =
            serde_json::from_value(json!({"response": 9007199254740993i64})).unwrap();
        assert!(compare(&expect, &json!(9007199254740993i64)).is_empty());
        assert!(!compare(&expect, &json!(9007199254740992i64)).is_empty());
    }

    #[test]
    fn statuses_compare_by_canonical_name() {
        assert!(status_matches(Some("NOT_FOUND"), Status::NotFound));
        assert!(!status_matches(Some("not_found"), Status::NotFound));
        assert!(status_matches(None, Status::Ok));
    }
}
