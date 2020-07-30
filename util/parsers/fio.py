#!/usr/bin/env python3

import sys
import json

with open(sys.argv[1]) as f:
	r = json.load(f)

res = {}
j = r['jobs'][0]
if j['read']['total_ios'] > 0:
	res['rlats'] = j['read']['lat_ns']['percentile']
	res['rlats']['max'] = j['read']['lat_ns']['max']
	res['rlats']['mean'] = j['read']['lat_ns']['mean']
	res['rbw'] = {'max': 0, 'mean': 0}
	res['riops'] = {'max': 0, 'mean': 0}
	res['rbw']['max'] = j['read']['bw_max']
	res['rbw']['mean'] = j['read']['bw_mean']
	res['riops']['max'] = j['read']['iops_max']
	res['riops']['mean'] = j['read']['iops_mean']
if j['write']['total_ios'] > 0:
	res['wlats'] = j['write']['lat_ns']['percentile']
	res['wlats']['max'] = j['write']['lat_ns']['max']
	res['wlats']['mean'] = j['write']['lat_ns']['mean']
	res['wbw'] = {'max': 0, 'mean': 0}
	res['wiops'] = {'max': 0, 'mean': 0}
	res['wbw']['max'] = j['write']['bw_max']
	res['wbw']['mean'] = j['write']['bw_mean']
	res['wiops']['max'] = j['write']['iops_max']
	res['wiops']['mean'] = j['write']['iops_mean']

print(json.dumps(res))
