#![allow(missing_docs)]

use std::env;
use std::io::BufRead;
use std::path::PathBuf;
use std::process::Command;
use std::thread;
use std::time::Duration;

use opensysml::{Complex, Connection, Error, EvalOptions, Value};

fn service_or_skip() -> Option<Connection> {
    match Connection::private() {
        Ok(connection) => Some(connection),
        Err(error) => {
            if env::var("OPENSYSML_REQUIRE_SERVICE").ok().as_deref() == Some("1") {
                panic!("required sysml-grpc service unavailable: {error}");
            }
            eprintln!("skipping service-backed Rust client test: {error}");
            None
        }
    }
}

#[test]
fn parse_eval_and_navigation() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = connection
        .parse_content("package Demo {}", &Default::default())
        .unwrap_or_else(|error| panic!("parse failed: {error}"));
    assert!(!model.hash().is_empty());
    assert_eq!(
        model
            .eval("2 + 2")
            .unwrap_or_else(|error| panic!("evaluation failed: {error}")),
        Value::Integer(4)
    );
    assert_eq!(
        model.root().expect("parsed model has a root").kind(),
        "RootNamespace"
    );
    let adopted = connection.model_by_hash(model.hash());
    assert_eq!(adopted.hash(), model.hash());
}

#[test]
fn missing_capability_is_legible() {
    let error = Error::MissingCapability {
        capability: "strict_conformance".to_owned(),
        remedy: "upgrade the service".to_owned(),
    };
    assert!(error.to_string().contains("strict_conformance"));
    assert!(error.to_string().contains("upgrade"));
}

#[test]
fn start_failure_is_legible() {
    let error = Error::ServiceStart("binary exited before reporting an address".to_owned());
    assert!(error.to_string().contains("service start failed"));
    assert!(error.to_string().contains("reporting an address"));
}

#[test]
fn unknown_model_hash_is_a_transport_service_error() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let result = connection.diagnostics("definitely-unknown-model-hash");
    assert!(matches!(
        result,
        Err(Error::Service {
            status: opensysml::Status::NotFound,
            ..
        })
    ));
}

#[test]
fn unknown_model_hash_symbol_is_a_transport_service_error() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let result = connection
        .model_by_hash("no-such-model")
        .symbol("Demo::missing");
    assert!(matches!(
        result,
        Err(Error::Service {
            status: opensysml::Status::NotFound,
            ..
        })
    ));
}

#[test]
fn unknown_model_hash_evaluation_is_a_transport_service_error() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let result = connection.model_by_hash("no-such-model").eval("2 + 2");
    assert!(matches!(
        result,
        Err(Error::Service {
            status: opensysml::Status::NotFound,
            ..
        })
    ));
}

#[test]
fn in_band_evaluation_errors_remain_model_errors() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = match connection.parse_content("package ErrorTest {}", &Default::default()) {
        Ok(model) => model,
        Err(error) => panic!("parse failed: {error}"),
    };
    assert!(matches!(model.eval("not valid ((("), Err(Error::Model(_))));
}

#[test]
fn instantiate_decodes_feature_values() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = match connection.parse_content(
        "package Demo { part def Car { attribute mass : Integer = 3; } part car : Car; }",
        &Default::default(),
    ) {
        Ok(model) => model,
        Err(error) => panic!("parse failed: {error}"),
    };
    let instance = match model.instantiate("Demo::car") {
        Ok(instantiation) => instantiation.instance,
        Err(error) => panic!("instantiation failed: {error}"),
    };
    assert_eq!(instance.type_symbol_id(), "Demo::car");
    assert!(instance.feature("mass").is_some());
}

#[test]
fn a_complex_number_is_one_value_with_both_parts() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    assert!(connection.capabilities().has("complex_values"));
    let model = match connection.parse_content(
        "package C {
            private import ScalarValues::*;
            private import ComplexFunctions::*;
            part def Signal {
                attribute z : Complex = rect(1.5, -2.0);
                attribute zs : Complex[2] = (rect(1.0, 2.0), rect(3.0, 4.0));
            }
        }",
        &Default::default(),
    ) {
        Ok(model) => model,
        Err(error) => panic!("parse failed: {error}"),
    };
    let options = EvalOptions {
        context: Some("C::Signal".to_owned()),
        subject: None,
    };
    let evaluated = match model.evaluate("z", &options) {
        Ok(evaluation) => evaluation.result,
        Err(error) => panic!("evaluation failed: {error}"),
    };
    assert_eq!(
        evaluated,
        Value::Complex(Complex {
            real: 1.5,
            imaginary: -2.0
        })
    );
    let instance = match model.instantiate("C::Signal") {
        Ok(instantiation) => instantiation.instance,
        Err(error) => panic!("instantiation failed: {error}"),
    };
    let z = instance.feature("z").expect("z is materialized");
    assert_eq!(
        z.value(),
        Some(&Value::Complex(Complex {
            real: 1.5,
            imaginary: -2.0
        }))
    );
    let zs = instance.feature("zs").expect("zs is materialized");
    assert_eq!(
        zs.values(),
        [
            Value::Complex(Complex {
                real: 1.0,
                imaginary: 2.0
            }),
            Value::Complex(Complex {
                real: 3.0,
                imaginary: 4.0
            }),
        ]
    );
}

#[test]
fn blocking_calls_work_inside_a_runtime() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let runtime = match tokio::runtime::Builder::new_current_thread().build() {
        Ok(runtime) => runtime,
        Err(error) => panic!("runtime creation failed: {error}"),
    };
    let result = runtime.block_on(async move {
        connection.parse_content("package RuntimeTest {}", &Default::default())
    });
    assert!(
        result.is_ok(),
        "blocking client call inside runtime failed: {result:?}"
    );
}

#[cfg(unix)]
#[test]
fn killing_the_parent_leaves_no_private_service() {
    let Some(binary) = env::var_os("CARGO_BIN_EXE_child_probe")
        .map(PathBuf::from)
        .or_else(|| {
            std::env::current_exe().ok().and_then(|executable| {
                executable
                    .ancestors()
                    .map(|directory| {
                        directory.join("examples").join(if cfg!(windows) {
                            "child_probe.exe"
                        } else {
                            "child_probe"
                        })
                    })
                    .find(|candidate| candidate.is_file())
            })
        })
    else {
        eprintln!("skipping SIGKILL lifecycle test: child_probe was not built");
        return;
    };
    let mut probe = match Command::new(binary)
        .stdout(std::process::Stdio::piped())
        .spawn()
    {
        Ok(probe) => probe,
        Err(error) => {
            if env::var("OPENSYSML_REQUIRE_SERVICE").ok().as_deref() == Some("1") {
                panic!("could not start lifecycle probe: {error}");
            }
            eprintln!("skipping SIGKILL lifecycle test: {error}");
            return;
        }
    };
    let Some(stdout) = probe.stdout.take() else {
        panic!("lifecycle probe stdout was not piped");
    };
    let mut lines = std::io::BufReader::new(stdout).lines();
    let pid = match lines.next() {
        Some(Ok(line)) => match line.parse::<u32>() {
            Ok(pid) => pid,
            Err(error) => panic!("lifecycle probe returned invalid pid: {error}"),
        },
        Some(Err(error)) => panic!("could not read lifecycle probe: {error}"),
        None => panic!("lifecycle probe exited without reporting a pid"),
    };
    let kill_status = Command::new("kill")
        .args(["-KILL", &probe.id().to_string()])
        .status();
    assert!(
        matches!(kill_status, Ok(status) if status.success()),
        "could not SIGKILL lifecycle probe: {kill_status:?}"
    );
    let _ = probe.wait();
    for _ in 0..50 {
        if !PathBuf::from(format!("/proc/{pid}")).exists() {
            return;
        }
        thread::sleep(Duration::from_millis(100));
    }
    panic!("private service {pid} survived parent SIGKILL");
}
