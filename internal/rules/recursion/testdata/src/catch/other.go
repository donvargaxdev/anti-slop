package catch

func blue() { red() } // want "blue calls itself through a cycle"
