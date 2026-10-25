#!/usr/bin/env python3
"""Isolated updater integration tests. Temporary files; no host services or USB."""
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('updater', Path(__file__).with_name('update-prebuilt.py'))
updater = importlib.util.module_from_spec(spec)
spec.loader.exec_module(updater)

class FakeAPI:
    base = 'http://127.0.0.1:8787'
    def __init__(self, version='0.3.0-dev'):
        self.version = version
        self.token = 'fixture-token'
    def get(self, path):
        if path == '/healthz':
            return {'version': self.version}
        if path == '/api/backup':
            return {'remotes': [{'buttons': [{'enabled': False, 'last_received': 'new'}]}]}
        return {'version': self.version, 'receiver': {'state': 'connected'}}
    def request(self, path, authenticated=True):
        return (200, self.get(path)) if authenticated and self.token == 'fixture-token' else (401, None)

class UpdateTests(unittest.TestCase):
    def test_listener_startup_wait_and_auth_failure(self):
        api = updater.API('127.0.0.1:8787', 'fixture-token')
        with patch.object(api, 'get', side_effect=[updater.URLError('starting'), {'ready': True}]), patch.object(updater.time, 'sleep'):
            self.assertEqual(api.wait_ready(), {'ready': True})
        with patch.object(api, 'get', side_effect=RuntimeError('Application /api/state returned HTTP 401')):
            with self.assertRaisesRegex(RuntimeError, '401'):
                api.wait_ready()

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        package = self.root / 'package'
        (package / 'bin').mkdir(parents=True)
        (package / 'bin/rf-button-manager').write_bytes(b'new binary')
        (package / 'rf-button-manager.git.bundle').write_bytes(b'fixture bundle')
        (package / 'release.json').write_text(json.dumps({'version': '0.3.0-dev', 'commit': 'fixture', 'binary_sha256': updater.sha(package / 'bin/rf-button-manager'), 'bundle_sha256': updater.sha(package / 'rf-button-manager.git.bundle')}))
        self.update = updater.Update(package, self.root, self.root / 'proc')
        process = self.root / 'proc/456'
        process.mkdir(parents=True)
        (process / 'comm').write_text('rtl_433\n')
        (process / 'status').write_text('PPid:\t123\n')
        (process / 'cgroup').write_text('0::/system.slice/rf-button-manager.service\n')
        u = self.update
        u.state.mkdir(parents=True)
        u.installed.parent.mkdir(parents=True)
        u.env.parent.mkdir(parents=True)
        u.installed.write_bytes(b'old binary')
        (u.state / 'manager.db').write_bytes(b'old database')
        (u.state / 'management.token').write_bytes(b'private fixture token')
        u.env.write_bytes(b'private fixture environment')
        u.token = (u.state / 'management.token').read_bytes()
        u.environment = u.env.read_bytes()
        u.config = {'remotes': [{'buttons': [{'enabled': False}]}]}
        u.api = FakeAPI()
        u.was_connected = True
        self.commands = []

    def host_command(self, *args):
        self.commands.append(args)
        if args[0] == 'cp':
            return subprocess.run(args, check=True, capture_output=True, text=True).stdout.strip()
        self.assertEqual(args[0], 'systemctl')
        self.assertIn(updater.UNIT, args)
        if args[1] == 'show':
            return '123'
        return 'active'

    def test_apply_deploys_actual_binary_and_checks_live_version(self):
        u = self.update
        with patch.object(updater, 'run', self.host_command), patch.object(updater.time, 'sleep'):
            u.apply()
        self.assertEqual(u.installed.read_bytes(), b'new binary')
        self.assertEqual((u.snapshot / 'binary').read_bytes(), b'old binary')
        self.assertEqual((u.snapshot / 'state/manager.db').read_bytes(), b'old database')
        self.assertEqual((u.state / 'management.token').read_bytes(), u.token)
        self.assertTrue(json.loads((u.snapshot / 'update-report.json').read_text())['live_version_verified'])
        self.assertEqual([c[1] for c in self.commands if c[:2] in [('systemctl', 'stop'), ('systemctl', 'start')]], ['stop', 'start'])

    def test_wrong_live_version_rolls_back_binary_and_database(self):
        u = self.update
        def failed_verify():
            (u.state / 'manager.db').write_bytes(b'changed during failed attempt')
            u.api.version = '0.2.1-dev'
            u.await_live(u.manifest['version'], seconds=0)
        with patch.object(updater, 'run', self.host_command), patch.object(u, 'verify', failed_verify):
            with self.assertRaisesRegex(RuntimeError, 'Live service'):
                u.apply()
        self.assertEqual(u.installed.read_bytes(), b'old binary')
        self.assertEqual((u.state / 'manager.db').read_bytes(), b'old database')
        self.assertEqual((u.snapshot / 'failed-state/manager.db').read_bytes(), b'changed during failed attempt')
        self.assertFalse((u.snapshot / 'update-report.json').exists())

    def test_cli_apply_is_dispatched_and_dry_run_does_not_install(self):
        for args, count in [([], 0), (['--apply'], 1)]:
            with patch.object(updater.os, 'geteuid', return_value=0), patch.object(updater, 'Update') as factory:
                updater.main(args)
                factory.return_value.preflight.assert_called_once()
                self.assertEqual(factory.return_value.apply.call_count, count)

    def test_corrupt_binary_stops_before_any_host_operation(self):
        self.update.binary.write_bytes(b'corrupt')
        with self.assertRaisesRegex(RuntimeError, 'checksum mismatch'):
            updater.Update(self.update.package, self.root)

if __name__ == '__main__':
    unittest.main()
