# goads
Go (Golang) wrapper for the Twincat ADS library, enabling you to effortlessly read and write PLC variables while also managing device notifications in your Go applications.

# Read a Variable

```go
package main

import (
	"fmt"
	"github.com/dinceruur/goads"
)

func main() {
	plc := goads.NewPLC()

	var boolVariable bool
	if err := plc.ReadByName("Main.BoolVariable", &boolVariable); err != nil {
		fmt.Println(err)
	}
	fmt.Println(boolVariable)
}
```

# Write a Variable

```go
package main

import (
	"fmt"
	"github.com/dinceruur/goads"
)

func main() {
	plc := goads.NewPLC()

	var boolVariable bool
	boolVariable = true
	if err := plc.WriteByName("Main.BoolVariable", &boolVariable); err != nil {
		fmt.Println(err)
	}
}
```
