use serde_json::Value;

pub const MODEL_HASH: &str = "${model_hash}";
pub const VERSION: &str = "${version}";
pub const PATH: &str = "${path}";

pub fn normalize(value: &mut Value, model_hash: &str) {
    normalize_inner(value, model_hash);
}

fn normalize_inner(value: &mut Value, model_hash: &str) {
    match value {
        Value::Object(object) => {
            let is_instance = object.contains_key("type_symbol_id")
                && object.contains_key("feature_values")
                && object.contains_key("id");
            let is_server_info =
                object.contains_key("capabilities") && object.contains_key("version");
            for (key, child) in object.iter_mut() {
                if is_server_info && key == "version" {
                    *child = Value::String(VERSION.to_owned());
                } else if is_instance && key == "id" {
                    // Runtime ids are labelled in one pass after all other
                    // normalization, where the response-wide map is shared.
                } else if key == "instance_id" {
                    // See compare::label_instance_ids.
                } else {
                    normalize_inner(child, model_hash);
                }
            }
        }
        Value::Array(items) => {
            for item in items {
                normalize_inner(item, model_hash);
            }
        }
        Value::String(text) => {
            if !model_hash.is_empty() && text == model_hash {
                *text = MODEL_HASH.to_owned();
            } else if is_absolute_path(text) {
                *text = PATH.to_owned();
            }
        }
        _ => {}
    }
}

fn is_absolute_path(value: &str) -> bool {
    value.starts_with('/')
        || (value.len() > 2 && value.as_bytes()[1] == b':' && value.as_bytes()[2] == b'\\')
}
