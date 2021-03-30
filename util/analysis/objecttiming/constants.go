package main

// Flags for each action
const (
	ParseCreate = 1 << iota
	ParseScale
	ParsePVCPod
	ParseDelete
	ParseResize
)

// Constants for the string names of each action
const (
	strCreate = "create"
	strScale  = "scale"
	strDelete = "delete"
	strResize = "resize"
)
