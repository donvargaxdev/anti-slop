package catch

func check(a, b, c, d bool) {
	if a && b && c && d {
	} else if a || b || c || d {
	}
	for a || (b && c) || d {
		break
	}
	for i := 0; i < 1 && a && b && c; i++ {
	}
	if !(a || b || c || d) {
	}
	if func() bool {
		if a && b && c && d {
			return true
		}
		return false
	}() {
	}
}
