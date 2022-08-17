package util

func GetConfig() string {
	return "conf.json"
}

func Btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
