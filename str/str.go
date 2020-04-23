package str

func StringInSlice(s string, ss []string) bool {
	for _, sss := range ss {
		if s == sss {
			return true
		}
	}

	return false
}

func RemoveSlice(source []string, toRemove []string) (result []string) {
	for _, s := range source {
		if !StringInSlice(s, toRemove) {
			result = append(result, s)
		}
	}

	return
}

func IndexOf(source []string, elem string) int {
	for i, s := range source {
		if elem == s {
			return i
		}
	}

	return -1
}
