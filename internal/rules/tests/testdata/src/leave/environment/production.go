package environment

import "os"

func Configure() { _ = os.Setenv("MODE", "prod") }
