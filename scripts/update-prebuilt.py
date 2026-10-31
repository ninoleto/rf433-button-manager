#!/usr/bin/env python3
"""Update an existing native installation. Dry run by default; --apply deploys.

No compiler, package installation, USB reset, or webhook invocation. The live
HTTP version must match the package before this program reports success.
"""
import argparse
import base64
import ssl
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
from urllib.request import Request, build_opener, ProxyHandler, HTTPRedirectHandler, HTTPSHandler

UNIT = 'rf433-button-manager.service'

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
    def __init__(self, address, token, state=None):
        self.base, self.token = 'http://' + address, token
        handlers = [ProxyHandler({}), NoRedirect()]
        if state is not None and (state / 'network.json').exists():
            network = json.loads((state / 'network.json').read_text())
            host = network['address']
            ip = ipaddress.ip_address(host)
            if ip.is_unspecified or ip.is_multicast or not (ip.is_private or ip in ipaddress.ip_network('100.64.0.0/10')):
                raise RuntimeError('Saved network listener is not private')
            host = '[' + host + ']' if ':' in host else host
            if network['https']:
                cert = json.loads((state / 'tls.json').read_text())
                context = ssl.create_default_context(cadata=base64.b64decode(cert['ca']).decode())
                handlers.append(HTTPSHandler(context=context))
                self.base = 'https://' + host + ':' + str(network['https_port'])
            elif network['http']:
                self.base = 'http://' + host + ':' + str(network['http_port'])
            else:
                raise RuntimeError('No enabled application listener')
        self.opener = build_opener(*handlers)

    def request(self, path, authenticated=True, method="GET", body=None):
        headers = {'Authorization': 'Bearer ' + self.token} if authenticated else {}
        data = None if body is None else json.dumps(body).encode()
        if body is not None: headers['Content-Type'] = 'application/json'
        try:
            with self.opener.open(Request(self.base + path, headers=headers, method=method, data=data), timeout=5) as response:
                return response.status, json.load(response)
        except HTTPError as exc:
            return exc.code, None

    def get(self, path):
        status, data = self.request(path)
        if status != 200:
            raise RuntimeError(f'Application {path} returned HTTP {status}')
        return data

    def actions(self, flags):
        current = self.get('/api/state')
        for remote in current['remotes']:
            for button in remote['buttons']:
                enabled = flags.get(button['id'], False)
                if button['enabled'] != enabled:
                    button['enabled'] = enabled
                    if self.request('/api/buttons/' + button['id'], method='PUT', body=button)[0] != 200:
                        raise RuntimeError('Could not preserve button enabled flags; consult the private update snapshot')

    def wait_ready(self, seconds=40):
        # The operator may have just restarted the service after a receiver
        # terminal test. Wait for its listener without changing any service.
        deadline = time.monotonic() + seconds
        while True:
            try:
                return self.get('/api/state')
            except (OSError, URLError):
                if time.monotonic() >= deadline:
                    raise RuntimeError('Application is not reachable; start rf433-button-manager.service and retry')
                time.sleep(1)

class Update:
    def __init__(self, package, root=Path('/'), proc=Path('/proc')):
        self.package = package
        self.root = root
        self.original_flags = {}
        self.migrating = (root / 'etc/systemd/system/rf-button-manager.service').exists()
        self.service_name = 'rf-button-manager.service' if self.migrating else UNIT
        self.renamed = False
        self.migration_moves = []
        self.old_home = None
        self.binary = package / 'bin/rf433-button-manager'
        self.installed = root / 'usr/local/bin/rf433-button-manager'
        self.state = root / 'var/lib/rf433-button-manager'
        self.env = root / 'etc/rf433-button-manager/manager.env'
        self.unit = root / 'etc/systemd/system' / self.service_name
        self.rules = root / 'etc/udev/rules.d/70-rf433-button-manager.rules'
        if self.migrating:
            self.installed = root / 'usr/local/bin/rf-button-manager'
            self.state = root / 'var/lib/rf-button-manager'
            self.env = root / 'etc/rf-button-manager/manager.env'
            self.rules = root / 'etc/udev/rules.d/70-rf-button-manager.rules'
        self.backups = root / 'var/lib/rf433-button-manager-upgrades'
        self.snapshot = None
        self.proc = proc
        self.manifest = json.loads((package / 'release.json').read_text())
        if sha(self.binary) != self.manifest['binary_sha256']:
            raise RuntimeError('Package executable checksum mismatch')
        if sha(package / 'rf433-button-manager.git.bundle') != self.manifest['bundle_sha256']:
            raise RuntimeError('Source bundle checksum mismatch')

    def preflight(self):
        if platform.system() != 'Linux' or platform.machine() != 'x86_64':
            raise RuntimeError('This package requires Linux x86_64')
        if run(str(self.binary), '--version') != self.manifest['version']:
            raise RuntimeError('Package executable version mismatch')
        if self.migrating:
            for relative in ('usr/local/bin/rf433-button-manager', 'var/lib/rf433-button-manager', 'etc/rf433-button-manager', 'etc/systemd/system/rf433-button-manager.service', 'etc/udev/rules.d/70-rf433-button-manager.rules'):
                if (self.root / relative).exists() or (self.root / relative).is_symlink():
                    raise RuntimeError('Both old and new installation paths exist; preserve them and resolve the conflict before migration')
            self.old_home = run('getent', 'passwd', 'rfbutton').split(':')[5]
            if self.old_home != '/var/lib/rf-button-manager':
                raise RuntimeError('Service account has a custom home; review it before migration')
            if (self.unit.parent / (self.service_name + '.d')).exists():
                raise RuntimeError('Legacy service has custom overrides; preserve and review them before migration')
            # A name migration must not discard operator changes to the unit.
            expected = (self.package / 'deploy' / UNIT).read_text().replace('rf433-button-manager', 'rf-button-manager')
            normalize = lambda text: '\n'.join(line for line in text.splitlines() if line.strip() and not line.lstrip().startswith('#'))
            actual = self.unit.read_text().replace('Description=RF Button Manager', 'Description=RF433 Button Manager')
            if normalize(actual) != normalize(expected):
                raise RuntimeError('Legacy service was customized; preserve and review it before migration')
        for path in (self.installed, self.state, self.env, self.env.parent, self.unit, self.rules, self.backups):
            if path.is_symlink():
                raise RuntimeError('Application path is a symlink; preserved for review')
        if not self.state.is_dir() or not any(marker in self.unit.read_text() for marker in ('Managed by RF Button Manager installer.', 'Managed by RF433 Button Manager installer.')):
            raise RuntimeError('Existing managed installation required; use install.sh for first installation')
        if any(p.is_symlink() for p in self.state.rglob('*')):
            raise RuntimeError('Application state contains symlinks; preserved for review')
        if self.backups.exists() and (self.backups.stat().st_mode & 0o077 or self.backups.stat().st_uid != 0):
            raise RuntimeError('Existing upgrade backup directory must be private (0700)')
        if run('systemctl', 'is-active', self.service_name) != 'active' or run('systemctl', 'is-enabled', self.service_name) != 'enabled':
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
        if ip.is_unspecified or ip.is_multicast or not (ip.is_private or ip in ipaddress.ip_network('100.64.0.0/10')) or not port.isdigit():
            raise RuntimeError('Expected an explicit private application listener')
        self.token = (self.state / 'management.token').read_bytes()
        self.environment = self.env.read_bytes()
        self.address = address
        self.api = API(address, self.token.decode().strip(), self.state)
        before = self.api.wait_ready()
        if before.get('learning') or before.get('actions_paused'):
            raise RuntimeError('Close learning in the GUI and wait for normal reception before updating')
        self.original_flags = {b['id']: b['enabled'] for r in before['remotes'] for b in r['buttons']}
        status, network = self.api.request('/api/manage/network')
        if status == 200 and network.get('pending'):
            raise RuntimeError('Confirm or cancel the pending network change before updating')
        if status not in (200, 404):
            raise RuntimeError('Could not verify whether a network change is pending; no installation changes made')
        if before.get('demo') or before.get('storage_error'):
            raise RuntimeError('Existing application reports demo mode or storage failure')
        self.config = stable(self.api.get('/api/backup'))
        self.was_connected = before['receiver']['state'] == 'connected'
        for directory in self.proc.glob('[0-9]*'):
            try:
                if (directory / 'comm').read_text().strip() in ('rtl_433', 'rtl_tcp', 'rtl_test'):
                    if not re.search(r'/' + re.escape(self.service_name) + r'(?:/|$)', (directory / 'cgroup').read_text(), re.M):
                        raise RuntimeError('Another receiver process is running (PID ' + directory.name + '); it was not stopped')
            except FileNotFoundError:
                continue
        size = sum(p.stat().st_size for p in self.state.rglob('*') if p.is_file())
        if shutil.disk_usage(self.state.parent).free < size * 2 + 100 * 1024 * 1024:
            raise RuntimeError('Insufficient space for application backup and update')
        print(f"Application-only update: {before['version']} -> {self.manifest['version']}", flush=True)
        print('Preserve mappings, webhooks, token, environment and enabled flags. Temporarily disable actions, back up stopped state, update and verify the LIVE version, then restore the original enabled flags. No webhook test calls or USB resets.', flush=True)
        if self.migrating:
            print('Migrate only application-owned paths from rf-button-manager to rf433-button-manager, including its service, USB rule filename and dedicated account home. Reload rules without triggering devices. Keep a rollback snapshot.', flush=True)

    def replace_binary(self, source):
        fd, name = tempfile.mkstemp(prefix='.rf433-button-manager.', dir=self.installed.parent)
        os.close(fd)
        try:
            shutil.copyfile(source, name)
            os.chmod(name, 0o755)
            os.replace(name, self.installed)
        finally:
            Path(name).unlink(missing_ok=True)

    def migrate_paths(self):
        self.old_paths = (self.installed, self.state, self.env, self.unit, self.rules)
        self.migration_created = []
        self.home_changed = False
        run('systemctl', 'disable', self.service_name)
        # The old service is stopped, so no database/WAL or token is copied live.
        targets = (
            (self.state, self.root / 'var/lib/rf433-button-manager'),
            (self.env.parent, self.root / 'etc/rf433-button-manager'),
            (self.installed, self.snapshot / 'retired-binary'),
            (self.installed.with_name(self.installed.name + '.previous'), self.snapshot / 'retired-previous-binary'),
            (self.unit, self.snapshot / 'retired-unit'),
            (self.rules, self.snapshot / 'retired-usb-rule'),
        )
        for old, new in targets:
            if old.exists():
                old.rename(new)
                self.migration_moves.append((old, new))
        self.installed = self.root / 'usr/local/bin/rf433-button-manager'
        self.state = self.root / 'var/lib/rf433-button-manager'
        self.env = self.root / 'etc/rf433-button-manager/manager.env'
        self.unit = self.root / 'etc/systemd/system' / UNIT
        self.rules = self.root / 'etc/udev/rules.d/70-rf433-button-manager.rules'
        self.service_name = UNIT
        for source, target in ((self.package / 'deploy' / UNIT, self.unit), (self.package / 'deploy/70-rf433-button-manager.rules', self.rules)):
            self.migration_created.append(target)
            shutil.copyfile(source, target)
            os.chmod(target, 0o644)
        # The numeric account/group and existing USB access remain unchanged.
        run('usermod', '--home', '/var/lib/rf433-button-manager', 'rfbutton')
        self.home_changed = True
        run('systemctl', 'daemon-reload')
        run('udevadm', 'control', '--reload-rules')
        run('systemctl', 'enable', UNIT)
        self.api = API(self.address, self.token.decode().strip(), self.state)
        self.renamed = True

    def rollback_paths(self):
        if not hasattr(self, 'old_paths'):
            return
        if self.service_name == UNIT:
            try:
                run('systemctl', 'disable', UNIT)
            except RuntimeError:
                pass
            self.installed.unlink(missing_ok=True)
        for path in getattr(self, 'migration_created', []):
            path.unlink(missing_ok=True)
        for old, new in reversed(self.migration_moves):
            if new.exists():
                new.rename(old)
        self.installed, self.state, self.env, self.unit, self.rules = self.old_paths
        self.service_name = 'rf-button-manager.service'
        if getattr(self, 'home_changed', False):
            run('usermod', '--home', self.old_home, 'rfbutton')
        run('systemctl', 'daemon-reload')
        run('udevadm', 'control', '--reload-rules')
        run('systemctl', 'enable', self.service_name)
        self.api = API(self.address, self.token.decode().strip(), self.state)

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
        pid = run('systemctl', 'show', self.service_name, '--property=MainPID', '--value')
        if not pid.isdigit() or int(pid) == 0:
            raise RuntimeError('Service has no live main process')
        for _ in range(20):
            time.sleep(1)
            if run('systemctl', 'is-active', self.service_name) != 'active':
                raise RuntimeError('Service stopped during verification')
            if run('systemctl', 'show', self.service_name, '--property=MainPID', '--value') != pid:
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
                    if not re.search(r'/' + re.escape(self.service_name) + r'(?:/|$)', cgroup, re.M):
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
        (self.snapshot / 'enabled-flags.json').write_text(json.dumps(self.original_flags))
        complete = False
        actions_changed = False
        if any(self.original_flags.values()):
            try:
                self.api.actions({})
                actions_changed = True
                self.config = stable(self.api.get('/api/backup'))
            except BaseException:
                self.api.actions(self.original_flags)
                raise
        try:
            run('systemctl', 'stop', self.service_name)
            run('cp', '-a', '--', str(self.state), str(self.snapshot / 'state'))
            run('cp', '-a', '--', str(self.installed), str(self.snapshot / 'binary'))
            run('cp', '-a', '--', str(self.env), str(self.snapshot / 'manager.env'))
            if self.migrating:
                run('cp', '-a', '--', str(self.unit), str(self.snapshot / 'service'))
                if self.rules.exists(): run('cp', '-a', '--', str(self.rules), str(self.snapshot / 'usb-rule'))
            complete = True
            if self.migrating: self.migrate_paths()
            self.replace_binary(self.binary)
            run('systemctl', 'start', self.service_name)
            self.await_live(self.manifest['version'])
            run('systemctl', 'restart', self.service_name)
            final = self.verify()
        except BaseException:
            if complete:
                try:
                    run('systemctl', 'stop', self.service_name)
                except RuntimeError:
                    # A failed migration can have removed the old unit before
                    # loading the new one. Restore its files before starting it.
                    if not self.migrating:
                        raise
                if self.migrating: self.rollback_paths()
                self.replace_binary(self.snapshot / 'binary')
                # Preserve the failed attempt too; restore only this app's state.
                self.state.rename(self.snapshot / 'failed-state')
                run('cp', '-a', '--', str(self.snapshot / 'state'), str(self.state))
                print('Update failed; restored the previous executable and stopped-state snapshot.', flush=True)
            run('systemctl', 'start', self.service_name)
            if actions_changed:
                try:
                    self.api.wait_ready()
                    self.api.actions(self.original_flags)
                except Exception:
                    print('Rollback retained disabled actions. Original flags are saved privately in ' + str(self.snapshot / 'enabled-flags.json'), flush=True)
            raise
        if actions_changed:
            self.api.actions(self.original_flags)
            check = self.api.get('/api/state')
            if {b['id']: b['enabled'] for r in check['remotes'] for b in r['buttons']} != self.original_flags:
                raise RuntimeError('Update is healthy but enabled-flag restoration needs review; private snapshot retained')
        report = {'version': final['version'], 'commit': self.manifest['commit'],
                  'gui_url': self.api.base, 'live_version_verified': True,
                  'stable_seconds': 20, 'authentication_enforced': True,
                  'configuration_token_environment_preserved': True,
                  'configuration_persisted_after_restart': True,
                  'enabled_flags_preserved': True, 'service': self.service_name, 'name_migrated': self.migrating, 'receiver_state': final['receiver']['state'],
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
