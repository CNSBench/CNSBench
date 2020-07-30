package utils

func ParseInterface(m interface{}, keys ...string) interface{} {
	lastVal := m
	for _, key := range keys {
		curMap := lastVal.(map[string]interface{})
		lastVal = curMap[key]
	}
	return lastVal
}
