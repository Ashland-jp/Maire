// models/stub.go
package models

import (
	"fmt"
	"time"
)

func Call(modelName, prompt string) string {
	time.Sleep(200 * time.Millisecond)
	short := prompt
	if len(prompt) > 130 {
		short = prompt[:130] + "…"
	}
	return fmt.Sprintf("[%s]\n%s\n\n→ Prompt length: %d", modelName, short, len(prompt))
}