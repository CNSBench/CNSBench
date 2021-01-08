package utils

func ParseInterface(m interface{}, keys ...string) interface{} {
	lastVal := m
	for _, key := range keys {
		curMap := lastVal.(map[string]interface{})
		lastVal = curMap[key]
	}
	return lastVal
}

func Contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func Remove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}
