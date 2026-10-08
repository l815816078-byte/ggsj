package util

// StringInSlice 判断字符串是否在切片中
func StringInSlice(target string, list []string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
