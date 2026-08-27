#!/usr/bin/env python3
"""buf local plugin bridging to the Python generators bundled with grpcio-tools.

grpcio-tools ships `python`, `pyi` and `grpc_python` only as builtins of its
bundled protoc, never as standalone protoc-gen-* binaries, so buf cannot invoke
them directly. The generator is selected by the plugin parameter, for example
`opt: grpc_python`.

Two normalizations keep the output identical to a direct protoc invocation:
  * json_name fields that buf populates redundantly are dropped, since protoc
    only serializes explicitly declared ones into the embedded descriptor;
  * same-directory `import x_pb2` lines are rewritten to package-relative
    imports, which the gRPC generator cannot emit itself.
"""

import os
import re
import subprocess
import sys
import tempfile

from google.protobuf import descriptor_pb2
from google.protobuf.compiler import plugin_pb2

GENERATORS = ("python", "pyi", "grpc_python")

_SIBLING_IMPORT = re.compile(r"^import (\w+_pb2) as (\w+)$", re.MULTILINE)


def relativize_imports(source: str) -> str:
    return _SIBLING_IMPORT.sub(r"from . import \1 as \2", source)


def default_json_name(field_name: str) -> str:
    head, *rest = field_name.split("_")
    return head + "".join(part[:1].upper() + part[1:] for part in rest)


def strip_default_json_names(descriptors: descriptor_pb2.FileDescriptorSet) -> None:
    """Clear json_name where it matches the value protoc would compute itself."""

    def visit(message: descriptor_pb2.DescriptorProto) -> None:
        for field in message.field:
            if field.json_name == default_json_name(field.name):
                field.ClearField("json_name")
        for nested in message.nested_type:
            visit(nested)

    for file in descriptors.file:
        for message in file.message_type:
            visit(message)
        for extension in file.extension:
            if extension.json_name == default_json_name(extension.name):
                extension.ClearField("json_name")


def generate(request: plugin_pb2.CodeGeneratorRequest) -> plugin_pb2.CodeGeneratorResponse:
    response = plugin_pb2.CodeGeneratorResponse()
    response.supported_features = plugin_pb2.CodeGeneratorResponse.FEATURE_PROTO3_OPTIONAL
    generator = request.parameter
    if generator not in GENERATORS:
        response.error = f"unknown generator {generator!r}, expected one of {', '.join(GENERATORS)}"
        return response
    descriptors = descriptor_pb2.FileDescriptorSet(file=request.proto_file)
    strip_default_json_names(descriptors)
    with tempfile.TemporaryDirectory() as tmp:
        image = os.path.join(tmp, "image.binpb")
        out = os.path.join(tmp, "out")
        os.mkdir(out)
        with open(image, "wb") as handle:
            handle.write(descriptors.SerializeToString())
        result = subprocess.run(
            [
                sys.executable,
                "-m",
                "grpc_tools.protoc",
                f"--descriptor_set_in={image}",
                f"--{generator}_out={out}",
                *request.file_to_generate,
            ],
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            response.error = result.stderr.strip() or "grpc_tools.protoc failed"
            return response
        for root, _, names in os.walk(out):
            for name in sorted(names):
                path = os.path.join(root, name)
                with open(path, encoding="utf-8") as handle:
                    content = handle.read()
                generated = response.file.add()
                generated.name = os.path.relpath(path, out)
                generated.content = relativize_imports(content)
    return response


def main() -> int:
    request = plugin_pb2.CodeGeneratorRequest.FromString(sys.stdin.buffer.read())
    sys.stdout.buffer.write(generate(request).SerializeToString())
    return 0


if __name__ == "__main__":
    sys.exit(main())
