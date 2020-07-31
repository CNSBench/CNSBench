import os
import requests
import json
import sys

metrics_q = {'from': 0,
	  'size': 10000,
	  'query': {
		'range': {
			'@timestamp': {
				'gte': 0,
				'lte': 0,
				'format': 'epoch_second'
			}
		}
	  },
	  'sort': [
		{
			'@timestamp': {
				'order': 'asc'
			}
		}
	  ]
	}

results_q = {'from': 0,
	  'size': 10000,
	  'query': {
		'range': {
			'CompletionTime': {
				'format': 'epoch_second'
			}
		}
	  },
	  'sort': [
		{
			'CompletionTime': {
				'order': 'asc'
			}
		}
	  ]
	}

def noWZero(m):
	return m['wbw']['max'] != 0 and m['wbw']['mean'] != 0 and m['wiops']['max'] != 0 and m['wiops']['mean'] != 0 and m['wlats']['max'] != 0 and m['wlats']['mean'] != 0 and m['wlats']['99.990000'] != 0

def noRZero(m):
	return m['rbw']['max'] != 0 and m['rbw']['mean'] != 0 and m['riops']['max'] != 0 and m['riops']['mean'] != 0 and m['rlats']['max'] != 0 and m['rlats']['mean'] != 0 and m['rlats']['99.990000'] != 0

def parseNodeMetric(m):
	mem = m['kubernetes']['node']['memory']['usage']['bytes']
	cpu = m['kubernetes']['node']['cpu']['usage']['nanocores']
	return {'mem': mem, 'cpu': cpu}

def parseNetMetric(m):
	rxb = m['system']['network']['in']['bytes']
	rxp = m['system']['network']['in']['packets']
	txb = m['system']['network']['out']['bytes']
	txp = m['system']['network']['out']['packets']
	return {'rxbytes': rxb, 'rxpackets': rxp, 'txbytes': txb, 'txpackets': txp}

def aggMetrics(samples):
	numHosts = len(samples[-1])
	c = 0
	metricsSum = {'mem': 0, 'cpu': 0}
	maxMetrics = {'mem': 0, 'cpu': 0}
	for s in samples:
		if len(s) < numHosts:
			continue
		c += 1
		curMetric = {'mem': 0, 'cpu': 0}
		for metrics in s.values():
			curMetric['mem'] += metrics['mem']
			curMetric['cpu'] += metrics['cpu']
		if curMetric['mem'] > maxMetrics['mem']:
			maxMetrics['mem'] = curMetric['mem']
		if curMetric['cpu'] > maxMetrics['cpu']:
			maxMetrics['cpu'] = curMetric['cpu']

		metricsSum['mem'] += curMetric['mem']
		metricsSum['cpu'] += curMetric['cpu']

	avgMetrics = {'mem': metricsSum['mem']/c, 'cpu': metricsSum['cpu']/c}
	return maxMetrics, avgMetrics

def main(args):
	headers = {'Content-Type': 'application/json'}
	es_address = sys.argv[1]
	url = 'http://{}/{}*/_search/'

	results_q['size'] = 10000
	results_q['query']['range']['CompletionTime']['gte'] = sys.argv[2]
	if len(sys.argv) > 3:
		results_q['query']['range']['CompletionTime']['lte'] = sys.argv[3]

	if os.path.exists('results'):
		with open('results') as f:
			results = json.load(f)
	else:
		results = {}

	r = requests.post(url.format(es_address, 'fiotest'), data=json.dumps(results_q), headers=headers)
	for i, h in enumerate(sorted(r.json()['hits']['hits'], key=lambda x: x['_source']['CompletionTime'])):
		ct = h['_source']['CompletionTime']
		st = h['_source']['StartTime']
		runtime = ct-st
		if runtime < 1000:
			continue

		if 'Name' in h['_source'] and 'baseline' in h['_source']['Name']:
			name = h['_source']['Name']
		else:
			workload = h['_source']['Spec']['actions'][0]['createObjSpec']['workload']
			sc = h['_source']['Spec']['actions'][0]['createObjSpec']['storageClass']
			name = workload + '-' + sc

		if name not in results:
			results[name] = {'w': {'bwMax': [], 'bwAvg': [], 'iopsMax': [], 'iopsAvg': [], 'latMax': [], 'latAvg': [], 'latP9999': []}, 'r': {'bwMax': [], 'bwAvg': [], 'iopsMax': [], 'iopsAvg': [], 'latMax': [], 'latAvg': [], 'latP9999': []}, 'cpuMax': [], 'cpuAvg': [], 'memMax': [], 'memAvg': [], 'rxbytes': [], 'rxpackets': [], 'txbytes': [], 'txpackets': []}

		if len(h['_source']['Results']['WorkloadResults']) > 0:
			print(name)
			for jr in h['_source']['Results']['WorkloadResults']:
				print(jr['PodName'], jr['NodeName'])
				if 'wbw' in jr['Results'] and noWZero(jr['Results']):
					results[name]['w']['bwMax'].append(jr['Results']['wbw']['max'])
					results[name]['w']['bwAvg'].append(jr['Results']['wbw']['mean'])
					results[name]['w']['iopsMax'].append(jr['Results']['wiops']['max'])
					results[name]['w']['iopsAvg'].append(jr['Results']['wiops']['mean'])
					results[name]['w']['latMax'].append(jr['Results']['wlats']['max'])
					results[name]['w']['latAvg'].append(jr['Results']['wlats']['mean'])
					results[name]['w']['latP9999'].append(jr['Results']['wlats']['99.990000'])
				if 'rbw' in jr['Results'] and noRZero(jr['Results']):
					results[name]['r']['bwMax'].append(jr['Results']['rbw']['max'])
					results[name]['r']['bwAvg'].append(jr['Results']['rbw']['mean'])
					results[name]['r']['iopsMax'].append(jr['Results']['riops']['max'])
					results[name]['r']['iopsAvg'].append(jr['Results']['riops']['mean'])
					results[name]['r']['latMax'].append(jr['Results']['rlats']['max'])
					results[name]['r']['latAvg'].append(jr['Results']['rlats']['mean'])
					results[name]['r']['latP9999'].append(jr['Results']['rlats']['99.990000'])

		metrics_q['query']['range']['@timestamp']['gte'] = st
		metrics_q['query']['range']['@timestamp']['lte'] = ct
		lastTS = ''
		nodeResults = {}
		nodeResultSamples = []
		lastNetStats = {}
		startNetStats = {}
		while True:
			if lastTS != '':
				metrics_q['search_after'] = [lastTS]
			mr = requests.post(url.format(es_address, 'metricbeat'), data=json.dumps(metrics_q), headers=headers)
			if 'error' in mr.json().keys():
				print(mr.text)
				break

			metrics = mr.json()['hits']['hits']
			if len(metrics) == 0:
				break

			for m in metrics:
				host = m['_source']['host']['name']

				if m['_source']['metricset']['name'] == 'node':
					nodeResults[host] = parseNodeMetric(m['_source'])
					nodeResultSamples.append(dict(nodeResults))
				elif m['_source']['metricset']['name'] == 'network':
					lastNetStats[host] = parseNetMetric(m['_source'])
					if host not in startNetStats:
						startNetStats[host] = lastNetStats[host]

			print(metrics[0]['_source']['@timestamp'], metrics[-1]['_source']['@timestamp'])
			lastTS = metrics[-1]['_source']['@timestamp']
		if len(nodeResultSamples) > 0:
			print(len(nodeResultSamples))
			netResults = {'rxbytes': 0, 'rxpackets': 0, 'txbytes': 0, 'txpackets': 0}
			for host, stats in startNetStats.items():
				for stat, val in stats.items():
					diff = lastNetStats[host][stat]-val
					if diff < 0:
						print('!!! negative diff', startNetStats, lastNetStats)
					netResults[stat] += diff
			for k, v in netResults.items():
				results[name][k].append(v)

			maxMetrics, avgMetrics = aggMetrics(nodeResultSamples)
			results[name]['cpuMax'].append(maxMetrics['cpu'])
			results[name]['memMax'].append(maxMetrics['mem'])
			results[name]['cpuAvg'].append(avgMetrics['cpu'])
			results[name]['memAvg'].append(avgMetrics['mem'])

	with open('results', 'w') as f:
		json.dump(results, f)

if __name__ == '__main__':
	main(sys.argv)
