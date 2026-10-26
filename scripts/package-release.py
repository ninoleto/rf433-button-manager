#!/usr/bin/env python3
"""Package a tested prebuilt executable, clean main source and full Git history."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import zipfile

def output(*args, cwd=None):
    return subprocess.check_output(args, cwd=cwd).decode().strip()

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, default=Path('dist'))
    parser.add_argument('--notices', type=Path, required=True)
    args = parser.parse_args()
    repo = Path(__file__).resolve().parent.parent
    if output('git', 'status', '--porcelain', cwd=repo):
        raise SystemExit('Commit reviewed source first; packaging requires a clean checkout')
    commit = output('git', 'rev-parse', 'HEAD', cwd=repo)
    if output('git', 'rev-parse', 'main', cwd=repo) != commit:
        raise SystemExit('Package main must refer to the tested HEAD commit')
    binary = repo / 'bin/rf-button-manager'
    version = output(str(binary), '--version')
    if not version or any(c not in '0123456789abcdefghijklmnopqrstuvwxyz.-' for c in version):
        raise SystemExit('Unexpected version')
    args.output.mkdir(parents=True, exist_ok=True)
    target = args.output.resolve() / f'rf433-button-manager-{version}-linux-amd64.zip'
    if target.exists():
        raise SystemExit('Refusing to overwrite an existing release archive')
    with tempfile.TemporaryDirectory(prefix='rfb-release-') as scratch:
        root = Path(scratch) / 'rf-button-manager'
        root.mkdir()
        archive = subprocess.check_output(['git', 'archive', commit], cwd=repo)
        with tarfile.open(fileobj=io.BytesIO(archive)) as source:
            source.extractall(root, filter='data')
        (root / 'bin').mkdir(exist_ok=True)
        shutil.copy2(binary, root / 'bin/rf-button-manager')
        (root / 'bin/rf-button-manager').chmod(0o755)
        shutil.copy2(args.notices, root / 'THIRD_PARTY_NOTICES.txt')
        bundle = root / 'rf-button-manager.git.bundle'
        subprocess.run(['git', 'bundle', 'create', str(bundle), 'refs/heads/main', 'HEAD'], cwd=repo, check=True)
        manifest = {'version': version, 'commit': commit, 'binary_sha256': digest(binary),
                    'bundle_sha256': digest(bundle), 'platform': 'linux-amd64',
                    'hardware_validation': 'pending operator testing'}
        (root / 'release.json').write_text(json.dumps(manifest, indent=2) + '\n')
        files = sorted(p for p in root.rglob('*') if p.is_file())
        (root / 'SHA256SUMS').write_text(''.join(f'{digest(p)}  {p.relative_to(root).as_posix()}\n' for p in files))
        with zipfile.ZipFile(target, 'w', compression=zipfile.ZIP_DEFLATED, compresslevel=9) as release:
            for path in sorted(root.rglob('*')):
                if path.is_file():
                    release.write(path, path.relative_to(root.parent))
    print(json.dumps({'archive': str(target), 'sha256': digest(target), **manifest}, indent=2))

if __name__ == '__main__':
    main()
