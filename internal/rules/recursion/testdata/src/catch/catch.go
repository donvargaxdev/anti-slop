package catch

func direct() { // want "direct calls itself$"
	direct()
	direct()
}

func first()  { second() } // want "first calls itself through a cycle"
func second() { first() }  // want "second calls itself through a cycle"

func red()   { green() } // want "red calls itself through a cycle"
func green() { blue() }  // want "green calls itself through a cycle"

func generic[T any](value T) { // want "generic calls itself$"
	(generic[T])(value)
}

func pair[A, B any](a A, b B) { // want "pair calls itself$"
	pair[A, B](a, b)
}

type node[T any] struct{}

func (n *node[T]) walk() { // want "walk calls itself$"
	(*node[T]).walk(n)
}

type edge struct{}

func (e edge) follow() { e.back() }   // want "follow calls itself through a cycle"
func (e edge) back()   { e.follow() } // want "back calls itself through a cycle"
