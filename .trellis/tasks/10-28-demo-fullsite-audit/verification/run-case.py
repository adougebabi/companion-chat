"""Run an existing real regression against this task's fresh stack only."""
import os,re,subprocess,sys
from pathlib import Path
private=Path('/private/tmp/fluctlight-demo-audit-20261028')
case=sys.argv[1]
if not re.fullmatch(r'l\d\d-[a-z-]+',case): raise SystemExit('Invalid case name')
script=Path('scripts/e2e/v3-regression')/(case+'.mjs')
if not script.is_file(): raise SystemExit('Missing existing case')
values=dict(line.split('=',1) for line in (private/'runtime.env').read_text().splitlines() if line and not line.startswith('#'))
env=os.environ.copy();env.update(FLUCTLIGHT_E2E_BASE_URL='http://127.0.0.1:18480',FLUCTLIGHT_E2E_ORIGIN='http://127.0.0.1:18480',FLUCTLIGHT_E2E_COOKIE_FILE=str(private/'cookies.txt'),FLUCTLIGHT_E2E_ISOLATED='1',FLUCTLIGHT_E2E_TIMEOUT_MS='600000',GO_CORE_TEST_DATABASE_URL='postgres://'+values['POSTGRES_USER']+':'+values['POSTGRES_PASSWORD']+'@127.0.0.1:55438/postgres?sslmode=disable',GOCACHE=str(private/'go-cache'))
with (private/(case+'.log')).open('w') as log:
 result=subprocess.run(['node',str(script),'--live','--run-dir',str(private/'runtime-business')],env=env,stdout=log,stderr=subprocess.STDOUT)
print(case,'exit',result.returncode,'report:',private/'runtime-business'/(case[:3].upper()+'.json'))
raise SystemExit(result.returncode)
