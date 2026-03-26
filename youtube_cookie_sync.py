#!/usr/bin/env python3
import json
import os
import subprocess
import sys
import urllib.request

SERVER = os.environ.get('MEOW_SERVER_URL', 'https://jd87.994938.xyz')
TOKEN = os.environ.get('MEOW_BEARER_TOKEN', '')
BROWSER = os.environ.get('YOUTUBE_COOKIE_BROWSER', 'chrome')
COOKIE_TXT = os.environ.get('YOUTUBE_COOKIE_FILE', os.path.expanduser('~/youtube-cookies.txt'))


def run(cmd):
    print('+', ' '.join(cmd))
    subprocess.check_call(cmd)


def export_cookies():
    # 需要本机已安装 yt-dlp，且浏览器保持 YouTube 登录态
    run([
        'yt-dlp',
        '--cookies-from-browser', BROWSER,
        '--cookies', COOKIE_TXT,
        '--skip-download',
        'https://www.youtube.com/watch?v=dQw4w9WgXcQ',
    ])


def push_cookies():
    with open(COOKIE_TXT, 'r', encoding='utf-8') as f:
        content = f.read()

    data = json.dumps({'content': content}).encode('utf-8')
    req = urllib.request.Request(
        SERVER.rstrip('/') + '/api/admin/youtube-cookie/update',
        data=data,
        method='POST',
        headers={
            'Content-Type': 'application/json',
            **({'Authorization': 'Bearer ' + TOKEN} if TOKEN else {}),
        },
    )
    with urllib.request.urlopen(req) as resp:
        print(resp.read().decode('utf-8', 'replace'))


if __name__ == '__main__':
    try:
        export_cookies()
        push_cookies()
        print('cookie sync ok')
    except Exception as e:
        print('cookie sync failed:', e, file=sys.stderr)
        sys.exit(1)
