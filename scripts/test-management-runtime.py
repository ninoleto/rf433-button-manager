#!/usr/bin/env python3
"""Exercise the built demo executable over HTTP/TLS. No USB or real webhooks."""
import json
import os
from pathlib import Path
import socket
import ssl
import subprocess
import tempfile
import time
from urllib.error import HTTPError, URLError
from urllib.request import Request, build_opener, ProxyHandler, HTTPSHandler

binary = Path(os.environ.get('RFB_MANAGEMENT_BINARY', Path(__file__).resolve().parent.parent / 'bin/rf433-button-manager')).resolve()

def free_port():
    with socket.socket() as s:
        s.bind(('127.0.0.1', 0))
        return s.getsockname()[1]

def request(opener, base, path, token=None, body=None):
    headers = {'Authorization': 'Bearer ' + token} if token else {}
    data = None
    if body is not None:
        headers['Content-Type'] = 'application/json'
        data = json.dumps(body).encode()
    try:
        with opener.open(Request(base+path, data=data, headers=headers), timeout=3) as response:
            return response.status, response.read()
    except HTTPError as error:
        return error.code, b''

def ready(opener, base):
    for _ in range(60):
        try:
            if request(opener, base, '/healthz')[0] == 200: return
        except (URLError, OSError): pass
        time.sleep(.1)
    raise RuntimeError('isolated management server did not become ready')

def stop(process):
    process.terminate()
    try: process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        process.kill(); process.wait(timeout=2)

with tempfile.TemporaryDirectory(prefix='rf433-management-test-') as directory:
    root = Path(directory)
    http_port, https_port = free_port(), free_port()
    while https_port == http_port: https_port = free_port()
    base = 'http://127.0.0.1:' + str(http_port)
    secure = 'https://127.0.0.1:' + str(https_port)
    opener = build_opener(ProxyHandler({}))
    def start():
        return subprocess.Popen([str(binary), '--demo', '--listen', '127.0.0.1:'+str(http_port), '--data-dir', directory], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    process = start()
    try:
        ready(opener, base)
        token = (root / 'management.token').read_text().strip()
        assert request(opener, base, '/api/manage/network')[0] == 401
        assert request(opener, base, '/api/manage/network', 'bad-token')[0] == 401
        proposed = {'address':'127.0.0.1','http':False,'http_port':http_port,'https':True,'https_port':https_port}
        status, raw = request(opener, base, '/api/manage/network/stage', token, proposed)
        assert status == 200
        pending = json.loads(request(opener, base, '/api/manage/network', token)[1])['pending']
        assert request(opener, base, '/api/manage/network/confirm', token, {'id':pending['id']})[0] == 400
        assert request(opener, base, '/api/manage/certificate/prepare', token, {})[0] == 200
        status, certificate = request(opener, base, '/api/manage/certificate', token)
        assert status == 200 and b'PRIVATE KEY' not in certificate
        tls = build_opener(ProxyHandler({}), HTTPSHandler(context=ssl.create_default_context(cadata=certificate.decode())))
        ready(tls, secure)
        assert request(tls, secure, '/api/manage/network/confirm', token, {'id':pending['id']})[0] == 200
        status, raw = request(tls, secure, '/api/manage/token/rotate', token, {'confirm':True})
        assert status == 200
        new_token = json.loads(raw)['token']
        assert new_token != token and len(new_token) == 64
        assert request(tls, secure, '/api/state', token)[0] == 401
        assert request(tls, secure, '/api/state', new_token)[0] == 200
        assert (root / 'management.token').read_text().strip() == new_token
        assert (root / 'tls.json').stat().st_mode & 0o777 == 0o600
        stop(process)
        process = start()
        ready(tls, secure)
        network = json.loads(request(tls, secure, '/api/manage/network', new_token)[1])
        assert network['config'] == proposed and network['pending'] is None
        assert request(tls, secure, '/api/state', token)[0] == 401
        assert request(tls, secure, '/api/state', new_token)[0] == 200
        assert request(tls, secure, '/api/manage/certificate/renew', new_token, {})[0] == 200
        assert request(tls, secure, '/api/manage/certificate', new_token)[1] == certificate
        print('PASS: real demo executable enforces authentication, stages HTTPS, verifies CA trust, requires secure confirmation, rotates/revokes tokens, preserves settings across restart and renews without changing trust.')
        print('No USB receiver or external webhook was used; no browser rendering claim.')
    finally:
        stop(process)
