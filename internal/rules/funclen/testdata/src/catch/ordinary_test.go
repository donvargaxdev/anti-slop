package catch

import "testing"

func TestLong(t *testing.T) { // want "function body is 95 lines; limit is 70"
	{
		for _, value := range []int{
			1,
			2,
		} {
			t.Log(value)
		}
	}
	cases := []int{
		0,
		1,
		2,
		3,
		4,
		5,
		6,
		7,
		8,
		9,
		10,
		11,
		12,
		13,
		14,
		15,
		16,
		17,
		18,
		19,
		20,
		21,
		22,
		23,
		24,
		25,
		26,
		27,
		28,
		29,
		30,
		31,
		32,
		33,
		34,
		35,
		36,
		37,
		38,
		39,
		40,
		41,
		42,
		43,
		44,
		45,
		46,
		47,
		48,
		49,
		50,
		51,
		52,
		53,
		54,
		55,
		56,
		57,
		58,
		59,
		60,
		61,
		62,
		63,
		64,
		65,
		66,
		67,
		68,
		69,
		70,
	}

	if cases := cases[:1]; len(cases) != 0 {
		for _, value := range cases {
			t.Log(value)
		}
	}
	func(cases []int) {
		for _, value := range cases {
			t.Log(value)
		}
	}(cases[:1])
	for _, cases := range [][]int{cases[:1]} {
		for _, value := range cases {
			t.Log(value)
		}
	}
}
