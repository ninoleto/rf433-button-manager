#!/usr/bin/env python3
"""Fetch pinned public receiver recordings for optional offline tests. No USB access."""
import argparse
import hashlib
from pathlib import Path
from urllib.request import urlopen

COMMIT = '1aa8c29d6b9383739d1aeed495dbc5069b704392'
FILES = {
    'ev1527': ('EV1527-Universal-Remote/g018_433.92M_250k.cu8', '15bd7dd777d4832a21c88bce5bbf3b6df3dc428663e9ab0b30c399aef09efee1'),
    'pt2262': ('PT2262/01/gfile001.cu8', '0c0b5584357dc383c723c60d5643caefaca75c6873ae98b7172a6ef9de929cc6'),
    'akhan': ('akhan_remote/01/gfile001.cu8', '1582377ac3acd105b8d25432340bd55cd003a43dbb9802265e545bf4e59805bf'),
    'ht680': ('HT680_remote/gfile001.cu8', 'd6a500525433659e8f9deccf2c5cc78b95fb875d14ce83372a29dee223465e86'),
}

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=Path)
    args = parser.parse_args()
    args.directory.mkdir(parents=True, exist_ok=True)
    for name, (source, digest) in FILES.items():
        target = args.directory / (name + '_433.92M_250k.cu8')
        data = target.read_bytes() if target.exists() else urlopen(
            f'https://raw.githubusercontent.com/merbanan/rtl_433_tests/{COMMIT}/tests/{source}', timeout=30).read(4 << 20)
        if hashlib.sha256(data).hexdigest() != digest:
            raise SystemExit(f'Checksum mismatch: {name}; no replacement written')
        if not target.exists():
            target.write_bytes(data)
        print(name + ': SHA-256 verified')

if __name__ == '__main__':
    main()
