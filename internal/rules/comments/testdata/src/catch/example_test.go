package catch_test

import "fmt"

// Example is documented here. // want "no tool reads"
func Example() {
	// Output: this comes before the real directive. // want "no tool reads"

	fmt.Println("hello")
	// Output: hello
}

func Example_noOutput() {
	// Output: this is followed by a different group. // want "no tool reads"

	// Ordinary comment. // want "no tool reads"
}

func exampleHelper() {
	// Output: this is an ordinary function. // want "no tool reads"
}
