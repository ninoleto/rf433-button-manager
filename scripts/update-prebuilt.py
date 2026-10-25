#!/usr/bin/env python3
"""Update an existing native installation. Dry run by default; --apply deploys.

No compiler, package installation, USB reset, or webhook invocation. The live
HTTP version must match the package before this program reports success.
"""
import argparse
import copy
import datetime
import hashlib
import ipaddress
import json
import os
from pathlib import Path
import platform
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
import time
from urllib.error import HTTPError, URLError
from urllib.request import Request, build_opener, ProxyHandler, HTTPRedirectHandler

UNIT = 'rf-button-manager.service'

def run(*args):
    result = subprocess.run(args, capture_output=True, text=True, timeout=60)
    if result.returncode:
        raise RuntimeError(f'{Path(args[0]).name} failed (exit {result.returncode}); details withheld to protect configuration')
    return result.stdout.strip()

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def stable(config):
    config = copy.deepcopy(config)
    for remote in config['remotes']:
        for button in remote['buttons']:
            button.pop('last_received', None)
    return config

class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        return None

class API:
    def __init__(self, address, token):
        self.base, self.token = 'http://' + address, token
        self.opener = build_opener(ProxyHandler({}), NoRedirect())

    def request(self, path, authenticated=True):
        headers = {'Authorization': 'Bearer ' + self.token} if authenticated else {}
        try:
            with self.opener.open(Request(self.base + path, headers=headers), timeout=5) as response:
                return response.status, json.load(response)
        except HTTPError as exc:
            return exc.code, None

    def get(self, path):
        status, data = self.request(path)
        if status != 200:
            raise RuntimeError(f'Application {path} returned HTTP {status}')
        return data

class Update:
    def __init__(self, package, root=Path('/'), proc=Path('/proc')):
        self.package = package
        self.binary = package / 'bin/rf-button-manager'
        self.installed = root / 'usr/local/bin/rf-button-manager'
        self.state = root / 'var/lib/rf-button-manager'
        self.env = root / 'etc/rf-button-manager/manager.env'
        self.unit = root / 'etc/systemd/system' / UNIT
        self.backups = root / 'var/lib/rf-button-manager-upgrades'
        self.snapshot = None
        self.proc = proc
        self.manifest = json.loads((package / 'release.json').read_text())
        if sha(self.binary) != self.manifest['binary_sha256']:
            raise RuntimeError('Package executable checksum mismatch')
        if sha(package / 'rf-button-manager.git.bundle') != self.manifest['bundle_sha256']:
            raise RuntimeError('Source bundle checksum mismatch')

    def preflight(self):
        if platform.system() != 'Linux' or platform.machine() != 'x86_64':
            raise RuntimeError('This package requires Linux x86_64')
        if run(str(self.binary), '--version') != self.manifest['version']:
            raise RuntimeError('Package executable version mismatch')
        for path in (self.installed, self.state, self.env, self.unit, self.backups):
            if path.is_symlink():
                raise RuntimeError('Application path is a symlink; preserved for review')
        if not self.state.is_dir() or 'Managed by RF Button Manager installer.' not in self.unit.read_text():
            raise RuntimeError('Existing managed installation required; use install.sh for first installation')
        if any(p.is_symlink() for p in self.state.rglob('*')):
            raise RuntimeError('Application state contains symlinks; preserved for review')
        if self.backups.exists() and (self.backups.stat().st_mode & 0o077 or self.backups.stat().st_uid != 0):
            raise RuntimeError('Existing upgrade backup directory must be private (0700)')
        if run('systemctl', 'is-active', UNIT) != 'active' or run('systemctl', 'is-enabled', UNIT) != 'enabled':
            raise RuntimeError('Expected the existing service to be active and enabled')
        address = '127.0.0.1:8787'
        for line in self.env.read_text().splitlines():
            if line.strip().startswith('RFB_LISTEN='):
                values = shlex.split(line.strip().split('=', 1)[1])
                if len(values) != 1:
                    raise RuntimeError('Cannot read application listener')
                address = values[0]
        host, port = address.rsplit(':', 1)
        ip = ipaddress.ip_address(host.strip('[]'))
        if ip.is_unspecified or not (ip.is_private or ip in ipaddress.ip_network('100.64.0.0/10')) or not port.isdigit():
            raise RuntimeError('Expected an explicit private application listener')
        self.token = (self.state / 'management.token').read_bytes()
        self.environment = self.env.read_bytes()
        self.api = API(address, self.token.decode().strip())
        before = self.api.get('/api/state')
        if before.get('learning') or before.get('actions_paused'):
            raise RuntimeError('Close learning in the GUI and wait for normal reception before updating')
        if any(b['enabled'] for r in before['remotes'] for b in r['buttons']):
            raise RuntimeError('Disable actions in the GUI before applying this development update')
        if before.get('demo') or before.get('storage_error'):
            raise RuntimeError('Existing application reports demo mode or storage failure')
        self.config = stable(self.api.get('/api/backup'))
        self.was_connected = before['receiver']['state'] == 'connected'
        size = sum(p.stat().st_size for p in self.state.rglob('*') if p.is_file())
        if shutil.disk_usage(self.state.parent).free < size * 2 + 100 * 1024 * 1024:
            raise RuntimeError('Insufficient space for application backup and update')
        print(f"Application-only update: {before['version']} -> {self.manifest['version']}", flush=True)
        print('Preserve state, token, environment, service, USB rule and account. Back up stopped state; replace only the executable; verify the LIVE version. No webhook calls or USB changes.', flush=True)

    def replace_binary(self, source):
        fd, name = tempfile.mkstemp(prefix='.rf-button-manager.', dir=self.installed.parent)
        os.close(fd)
        try:
            shutil.copyfile(source, name)
            os.chmod(name, 0o755)
            os.replace(name, self.installed)
        finally:
            Path(name).unlink(missing_ok=True)

    def await_live(self, version, seconds=40):
        deadline = time.monotonic() + seconds
        while time.monotonic() < deadline:
            try:
                health = self.api.get('/healthz')
                state = self.api.get('/api/state')
                if health['version'] == version and state['version'] == version:
                    if not self.was_connected or state['receiver']['state'] == 'connected':
                        return state
            except (OSError, URLError, ValueError, RuntimeError):
                pass
            time.sleep(1)
        raise RuntimeError('Live service did not reach the expected version/receiver state')

    def verify(self):
        self.await_live(self.manifest['version'])
        pid = run('systemctl', 'show', UNIT, '--property=MainPID', '--value')
        if not pid.isdigit() or int(pid) == 0:
            raise RuntimeError('Service has no live main process')
        for _ in range(20):
            time.sleep(1)
            if run('systemctl', 'is-active', UNIT) != 'active':
                raise RuntimeError('Service stopped during verification')
            if run('systemctl', 'show', UNIT, '--property=MainPID', '--value') != pid:
                raise RuntimeError('Service restarted during verification')
            if self.api.get('/healthz')['version'] != self.manifest['version']:
                raise RuntimeError('Live version changed during verification')
        if stable(self.api.get('/api/backup')) != self.config:
            raise RuntimeError('Configuration changed across update')
        if (self.state / 'management.token').read_bytes() != self.token or self.env.read_bytes() != self.environment:
            raise RuntimeError('Token or environment changed across update')
        if self.api.request('/api/state', False)[0] != 401:
            raise RuntimeError('Management authentication is not enforced')
        original_token = self.api.token
        try:
            self.api.token = 'invalid-update-verification-token'
            if self.api.request('/api/state')[0] != 401:
                raise RuntimeError('Invalid management token was accepted')
        finally:
            self.api.token = original_token
        final = self.api.get('/api/state')
        if final.get('storage_error') or final.get('demo'):
            raise RuntimeError('Updated application reports storage failure or demo mode')
        owned = []
        for directory in self.proc.glob('[0-9]*'):
            try:
                if (directory / 'comm').read_text().strip() != 'rtl_433':
                    continue
                status = (directory / 'status').read_text()
                parent = re.search(r'^PPid:\s+(\d+)', status, re.M)
                cgroup = (directory / 'cgroup').read_text()
                if parent and parent.group(1) == pid:
                    if not re.search(r'/rf-button-manager\.service(?:/|$)', cgroup, re.M):
                        raise RuntimeError('Receiver child is outside the application service')
                    owned.append(int(directory.name))
            except FileNotFoundError:
                continue
        if len(owned) > 1 or (final['receiver']['state'] == 'connected' and len(owned) != 1):
            raise RuntimeError('Receiver status does not match decoder process ownership')
        self.owned_receivers = owned
        return final

    def apply(self):
        self.backups.mkdir(mode=0o700, parents=True, exist_ok=True)
        stamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S.%fZ')
        self.snapshot = self.backups / stamp
        self.snapshot.mkdir(mode=0o700)
        complete = False
        run('systemctl', 'stop', UNIT)
        try:
            run('cp', '-a', '--', str(self.state), str(self.snapshot / 'state'))
            run('cp', '-a', '--', str(self.installed), str(self.snapshot / 'binary'))
            run('cp', '-a', '--', str(self.env), str(self.snapshot / 'manager.env'))
            complete = True
            self.replace_binary(self.binary)
            run('systemctl', 'start', UNIT)
            final = self.verify()
        except BaseException:
            if complete:
                run('systemctl', 'stop', UNIT)
                self.replace_binary(self.snapshot / 'binary')
                # Preserve the failed attempt too; restore only this app's state.
                self.state.rename(self.snapshot / 'failed-state')
                run('cp', '-a', '--', str(self.snapshot / 'state'), str(self.state))
                print('Update failed; restored the previous executable and stopped-state snapshot.', flush=True)
            run('systemctl', 'start', UNIT)
            raise
        report = {'version': final['version'], 'commit': self.manifest['commit'],
                  'gui_url': self.api.base, 'live_version_verified': True,
                  'stable_seconds': 20, 'authentication_enforced': True,
                  'configuration_token_environment_preserved': True,
                  'actions_disabled': True, 'receiver_state': final['receiver']['state'],
                  'owned_live_receiver_pids': self.owned_receivers,
                  'physical_button_test': 'pending browser test', 'snapshot': str(self.snapshot)}
        (self.snapshot / 'update-report.json').write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps(report, indent=2), flush=True)

def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apply', action='store_true')
    args = parser.parse_args(argv)
    if os.geteuid() != 0:
        raise RuntimeError('Run with sudo so the private application state can be checked (dry run is read-only)')
    os.umask(0o077)
    update = Update(Path(__file__).resolve().parent.parent)
    update.preflight()
    if args.apply:
        update.apply()
        print('Update verified. Refresh the GUI; the learning dialog must show ' + update.manifest['version'] + '.')
    else:
        print('Dry run complete. Use --apply to deploy this exact prebuilt executable.')

if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError):
        # Exception messages from parsers or system utilities can contain secrets.
        # Deliberate application errors above are safe and useful to the operator.
        error = sys.exc_info()[1]
        print('Stopped: ' + (str(error) if isinstance(error, RuntimeError) else type(error).__name__ + '; inspect application files locally'), file=sys.stderr)
        sys.exit(1)
