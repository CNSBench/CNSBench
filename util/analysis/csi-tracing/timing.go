package main

import "sort"

func sortRPCs(rpcs []CsiRPC) {
	sort.Sort(byResponseTime(rpcs))
}

type byResponseTime []CsiRPC

func (a byResponseTime) Len() int {
	return len(a)
}
func (a byResponseTime) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a byResponseTime) Less(i, j int) bool {
	return a[i].responseTime.Before(a[j].responseTime)
}
