package leave

func check(a, b, c, d bool, n int) bool {
	named := a && b && c && d
	if named || b && c {
	}
	if ready := a || b || c || d; ready {
	}
	if n+1 < (n*2)/3 && a && b {
	}
	for a || b || c {
		break
	}
	for {
		break
	}
	for _, item := range []bool{a && b && c && d} {
		_ = item
	}
	switch a || b || c || d {
	case true:
	}
	if func() bool { return a && b && c && d }() {
	}
	return a && b && c && d
}
