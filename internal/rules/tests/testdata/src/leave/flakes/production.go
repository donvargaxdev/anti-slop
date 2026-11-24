package flakes

func Retry(check func() error) {
	for range 3 {
		if err := check(); err == nil {
			return
		}
	}
}
