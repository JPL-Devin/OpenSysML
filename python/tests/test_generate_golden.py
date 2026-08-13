"""Golden and static-typing tests for the generated typed classes.

The golden file is `tests/golden/vehicle_types.py`, generated from
`internal/repl/testdata/vehicle_package.sysml`. Regenerate it with a running
service from the repository root:

    python -m pysysml.generate internal/repl/testdata/vehicle_package.sysml \
        -o python/tests/golden/vehicle_types.py
"""

import importlib.util
import os
import shutil
import subprocess
import sys
import textwrap
from pathlib import Path

import pytest

from pysysml.generate import generate_source
from pysysml.typed import TypedObject

PYTHON_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = PYTHON_ROOT.parent
GOLDEN = PYTHON_ROOT / "tests" / "golden" / "vehicle_types.py"
FIXTURE = REPO_ROOT / "internal" / "repl" / "testdata" / "vehicle_package.sysml"
REGENERATE = (
    "python -m pysysml.generate internal/repl/testdata/vehicle_package.sysml "
    "-o python/tests/golden/vehicle_types.py"
)


def mypy_available():
    """Whether mypy can be run through the current interpreter."""
    return importlib.util.find_spec("mypy") is not None


def run_mypy(cwd, *files):
    """Run mypy over ``files``, reporting errors in them only."""
    return subprocess.run(
        [
            sys.executable,
            "-m",
            "mypy",
            "--no-incremental",
            "--no-error-summary",
            # Dependencies are analyzed for their types, but only the files
            # under test are held to being error-free.
            "--follow-imports=silent",
            *[str(path) for path in files],
        ],
        cwd=cwd,
        env={"MYPYPATH": str(PYTHON_ROOT), "PATH": "/usr/bin:/bin", "HOME": str(cwd)},
        capture_output=True,
        text=True,
    )


def load_golden():
    """Import the committed golden module."""
    spec = importlib.util.spec_from_file_location("vehicle_types", GOLDEN)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def service_available():
    """Whether a sysml-grpc service can be reached on the default port."""
    import grpc

    from pysysml.proto import sysml_pb2, sysml_pb2_grpc

    try:
        channel = grpc.insecure_channel("localhost:50051")
        stub = sysml_pb2_grpc.SysMLServiceStub(channel)
        stub.GetDiagnostics(sysml_pb2.DiagnosticsRequest(model_hash="health_check"), timeout=2)
        channel.close()
        return True
    except grpc.RpcError as exc:
        return exc.code() == grpc.StatusCode.NOT_FOUND
    except Exception:
        return False


def test_golden_module_is_importable_and_typed():
    """The committed golden file imports and exposes typed views."""
    module = load_golden()
    assert issubclass(module.Vehicle, TypedObject)
    assert module.Vehicle.sysml_id == "Demo::Vehicle"
    assert module.Engine.sysml_id == "Demo::Engine"
    annotations = {
        "mass": module.Vehicle.mass.fget.__annotations__["return"],
        "engine": module.Vehicle.engine.fget.__annotations__["return"],
        "power": module.Engine.power.fget.__annotations__["return"],
    }
    assert annotations == {"mass": "float", "engine": "Engine", "power": "float"}


@pytest.mark.skipif(not mypy_available(), reason="mypy not installed")
def test_generated_code_is_mypy_clean_and_flags_misuse(tmp_path):
    """mypy accepts correct use of the golden classes and rejects misuse."""
    shutil.copy(GOLDEN, tmp_path / "vehicle_types.py")
    (tmp_path / "usage_ok.py").write_text(
        textwrap.dedent(
            """
            from pysysml.instance import Instance
            from vehicle_types import Vehicle


            def check(inst: Instance) -> float:
                v: Vehicle = Vehicle.from_instance(inst)
                return v.mass + v.engine.power
            """
        )
    )
    (tmp_path / "usage_bad.py").write_text(
        textwrap.dedent(
            """
            from pysysml.instance import Instance
            from vehicle_types import Vehicle


            def check(inst: Instance) -> None:
                v = Vehicle.from_instance(inst)
                v.mas
                v.mass + "x"
            """
        )
    )

    clean = run_mypy(tmp_path, "vehicle_types.py", "usage_ok.py")
    assert clean.returncode == 0, clean.stdout + clean.stderr

    misuse = run_mypy(tmp_path, "usage_bad.py")
    assert misuse.returncode != 0
    assert 'has no attribute "mas"' in misuse.stdout
    assert 'Unsupported operand types for + ("float" and "str")' in misuse.stdout


@pytest.mark.skipif(not mypy_available(), reason="mypy not installed")
def test_typed_codegen_modules_are_mypy_clean():
    """The modules generated code depends on type-check cleanly themselves."""
    result = run_mypy(
        PYTHON_ROOT,
        PYTHON_ROOT / "pysysml" / "typed.py",
        PYTHON_ROOT / "pysysml" / "typefacts.py",
        PYTHON_ROOT / "pysysml" / "generate.py",
    )
    assert result.returncode == 0, result.stdout + result.stderr


@pytest.mark.integration
@pytest.mark.skipif(not service_available(), reason="sysml-grpc service not running")
def test_golden_matches_live_generation():
    """Regenerating from the live service reproduces the committed golden file."""
    from pysysml import Connection

    with Connection(auto_start=False) as conn:
        model = conn.load(str(FIXTURE))
        source = generate_source(model)
    assert source == GOLDEN.read_text(), f"golden is stale; regenerate with: {REGENERATE}"


@pytest.mark.integration
@pytest.mark.skipif(not service_available(), reason="sysml-grpc service not running")
def test_generated_classes_read_a_live_instance(tmp_path):
    """The documented workflow works end to end against a live service."""
    output = tmp_path / "demo_types.py"
    env = dict(os.environ)
    # The CLI connects through the auto-start path, whose refcount lives under
    # $HOME; an isolated HOME keeps its exit from shutting the service down.
    env["HOME"] = str(tmp_path)
    result = subprocess.run(
        [sys.executable, "-m", "pysysml.generate", str(FIXTURE), "-o", str(output)],
        cwd=PYTHON_ROOT,
        capture_output=True,
        text=True,
        env=env,
    )
    assert result.returncode == 0, result.stderr

    from pysysml import Connection

    spec = importlib.util.spec_from_file_location("demo_types", output)
    demo_types = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(demo_types)

    with Connection(auto_start=False) as conn:
        model = conn.load(str(FIXTURE))
        inst = conn.instantiate("Demo::Vehicle", model.hash)
    vehicle = demo_types.Vehicle.from_instance(inst)
    assert vehicle.mass == 1500.0
    assert vehicle.engine.power == 300.0
