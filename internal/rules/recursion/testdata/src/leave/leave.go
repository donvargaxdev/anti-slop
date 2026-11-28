package leave

import "strings"

func first() { second() }

func second() {
	_ = strings.TrimSpace(" text ")
	_ = int(2)
	_ = len("text")
}

func ringA() { ringB() }
func ringB() { ringC() }
func ringC() { ringD() }
func ringD() { ringA() }

func indirect(f func(), callbacks []func()) {
	f()
	callbacks[0]()
}

func shadowed() {
	shadowed := func() {}
	shadowed()
}

func deferredCallback() func() {
	return func() { deferredCallback() }
}

type caller interface{ call() }
type receiver struct{ next caller }
type other struct{}

func (r receiver) call() { r.next.call() }
func (other) call()      {}

func forward(r other) { r.call() }
