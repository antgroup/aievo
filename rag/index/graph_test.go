package index

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestGraph(t *testing.T) {
	output, err := exec.Command("which", "python").Output()
	if err != nil {
		panic(err)
	}
	fmt.Println(string(output))
}
