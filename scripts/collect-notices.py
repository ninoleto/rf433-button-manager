#!/usr/bin/env python3
"""Collect pinned module license/notice files for release packaging (Python 3)."""
import argparse
import json
from pathlib import Path
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", default="go", help="Go executable")
    parser.add_argument("--output", default="dist/THIRD_PARTY_NOTICES.txt")
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    result = subprocess.run(
        [args.go, "list", "-m", "-json", "all"], cwd=root,
        check=True, capture_output=True, text=True,
    )
    decoder = json.JSONDecoder()
    remaining = result.stdout.lstrip()
    modules = []
    while remaining:
        module, end = decoder.raw_decode(remaining)
        modules.append(module)
        remaining = remaining[end:].lstrip()

    parts = [
        "RF433 Button Manager — Third-party dependency notices\n",
        "Collected from the pinned module source distributions used by this build.\n"
        "rtl_433 is installed separately and is not included in this executable.\n"
        "Generated C translations may carry additional notices in source headers;\n"
        "review upstream source and this collection before publishing a release.\n",
    ]
    missing = []
    goroot = Path(subprocess.run(
        [args.go, "env", "GOROOT"], cwd=root,
        check=True, capture_output=True, text=True,
    ).stdout.strip())
    toolchain = subprocess.run(
        [args.go, "version"], cwd=root,
        check=True, capture_output=True, text=True,
    ).stdout.strip()
    parts.append(f"\n{'=' * 72}\nGo standard library ({toolchain})\n")
    for name in ("LICENSE", "PATENTS"):
        notice = goroot / name
        if notice.is_file():
            parts.append(f"\n--- Go {name} ---\n{notice.read_text().rstrip()}\n")
        elif name == "LICENSE":
            missing.append("Go standard library: LICENSE missing from GOROOT")
    for module in sorted(modules, key=lambda m: m["Path"]):
        if module.get("Main"):
            continue
        path = module["Path"]
        directory = module.get("Dir")
        if not directory:
            missing.append(f"{path}: module directory missing (run go mod download all)")
            continue
        texts = [
            file for file in Path(directory).iterdir()
            if file.is_file() and any(
                word in file.name.lower() for word in ("license", "licence", "copying", "notice")
            )
        ]
        if not texts:
            missing.append(f"{path}: no top-level license/notice file")
            continue
        parts.append(f"\n{'=' * 72}\n{path} {module.get('Version', '')}\n")
        for file in sorted(texts):
            parts.append(f"\n--- {file.name} ---\n{file.read_text(errors='replace').rstrip()}\n")
    if missing:
        print("Cannot finish notice collection:\n" + "\n".join(missing), file=sys.stderr)
        return 1
    output = Path(args.output)
    if not output.is_absolute():
        output = root / output
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text("\n".join(parts), encoding="utf-8")
    print(output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
