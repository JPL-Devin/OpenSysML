#!/usr/bin/env python3
"""Collapse the duplicate blocks a -coverpkg profile carries, keeping the highest count.

`go test -coverpkg=./... ./...` writes one entry per block *per test binary*, so a block
appears once for every package whose tests ran — covered in some, zero in others.
`go tool cover` takes the maximum; a consumer that takes the last entry instead reads a
covered block as uncovered, so the profile is collapsed here rather than trusted to be
read the same way twice.

Usage: dedupe-coverage.py <profile>   (rewritten in place)
"""

import sys
from pathlib import Path


def main() -> int:
    if len(sys.argv) != 2:
        print(f"usage: {Path(sys.argv[0]).name} <profile>", file=sys.stderr)
        return 2

    path = Path(sys.argv[1])
    mode = None
    counts: dict[str, int] = {}
    order: list[str] = []

    try:
        content = path.read_text()
    except OSError as err:
        print(f"{path}: {err.strerror}", file=sys.stderr)
        return 1

    for lineno, line in enumerate(content.splitlines(), start=1):
        if not line.strip():
            continue
        if line.startswith("mode:"):
            mode = line
            continue
        try:
            block, statements, count = line.rsplit(" ", 2)
            key = f"{block} {statements}"
            hits = int(count)
        except ValueError:
            print(f"{path}:{lineno}: not a coverage block: {line}", file=sys.stderr)
            return 1
        if key not in counts:
            order.append(key)
            counts[key] = hits
        else:
            counts[key] = max(counts[key], hits)

    if mode is None:
        print(f"{path}: no mode line; not a Go coverage profile", file=sys.stderr)
        return 1

    body = "".join(f"{key} {counts[key]}\n" for key in order)
    path.write_text(f"{mode}\n{body}")
    print(f"{path}: {len(order)} block(s)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
