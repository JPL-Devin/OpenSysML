#![allow(missing_docs)]

#[cfg(unix)]
#[test]
fn private_start_failure_is_observed() {
    use std::env;

    let prior = env::var_os("OPENSYSML_GRPC_BINARY");
    env::set_var("OPENSYSML_GRPC_BINARY", "/bin/true");
    let result = opensysml::Connection::private();
    match prior {
        Some(value) => env::set_var("OPENSYSML_GRPC_BINARY", value),
        None => env::remove_var("OPENSYSML_GRPC_BINARY"),
    }
    let error = result.expect_err("an immediately exiting binary must fail startup");
    match error {
        opensysml::Error::ServiceStart(message) => {
            assert!(
                message.contains("code Some(0)") || message.contains("stderr tail"),
                "startup failure omitted exit evidence: {message}"
            );
        }
        other => panic!("unexpected startup error: {other}"),
    }
}
