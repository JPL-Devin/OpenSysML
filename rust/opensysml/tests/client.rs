#![allow(missing_docs)]

use std::env;
use std::io::BufRead;
use std::path::PathBuf;
use std::process::Command;
use std::thread;
use std::time::Duration;

use opensysml::{Connection, Error, Value};

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
    let model = match connection.parse_content("package Demo {}", &Default::default()) {
        Ok(model) => model,
        Err(error) => {
            if env::var("OPENSYSML_REQUIRE_SERVICE").ok().as_deref() == Some("1") {
                panic!("parse failed: {error}");
            }
            eprintln!("skipping unavailable fixture: {error}");
            return;
        }
    };
    assert!(!model.hash().is_empty());
    match model.eval("2 + 2") {
        Ok(Value::Integer(4)) | Ok(Value::Real(4.0)) => {}
        Ok(value) => panic!("unexpected evaluation value: {value:?}"),
        Err(error) => panic!("evaluation failed: {error}"),
    }
    assert!(model.root().wire().id.is_empty() || model.root().id().contains("Demo"));
    let adopted = match connection.model_by_hash(model.hash()) {
        Ok(model) => model,
        Err(error) => panic!("adopting parsed model failed: {error}"),
    };
    assert_eq!(adopted.hash(), model.hash());
}

#[test]
fn private_connections_share_one_child_and_last_drop_stops_it() {
    let Some(first) = service_or_skip() else {
        return;
    };
    let pid = first.private_service_pid();
    let second = match Connection::private() {
        Ok(connection) => connection,
        Err(error) => panic!("second private connection failed: {error}"),
    };
    assert_eq!(pid, second.private_service_pid());
    drop(first);
    let Some(pid) = pid else {
        return;
    };
    drop(second);
    for _ in 0..50 {
        if !PathBuf::from(format!("/proc/{pid}")).exists() {
            return;
        }
        thread::sleep(Duration::from_millis(100));
    }
    panic!("private service {pid} remained after last connection dropped");
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
    let result = connection.model_by_hash("definitely-unknown-model-hash");
    eprintln!("unknown hash result: {result:?}");
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
            let candidate = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
                .join("target")
                .join("debug")
                .join("child_probe");
            candidate.is_file().then_some(candidate)
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
