package filereaders

import (
	"fmt"
	"testing"
)


// Example usage - commented out since this is now a library package
func TestReader(t *testing.T) {
	reader := NewGeneralReader()
	filePath := "../../dataset/gaia/val_files/a3fbeb63-0e8c-4a11-bff6-0e3b484c3e9c.pptx"
	// filePath := "../../dataset/gaia/val_files/9b54f9d9-35ee-4a14-b62f-d130ea00317f.zip"
	content, err := reader.Read("describe the file", filePath)
	if err != nil {
		t.Fatalf("Error reading file: %v", err)
	}

	fmt.Println(content)
}
