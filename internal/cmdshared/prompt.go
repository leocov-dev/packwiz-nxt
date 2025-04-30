package cmdshared

import (
	"bufio"
	"fmt"
	"github.com/spf13/viper"
	"os"
	"strings"
)

func PromptYesNo(prompt string) bool {
	fmt.Print(prompt)
	if viper.GetBool("non-interactive") {
		fmt.Println("Y (non-interactive mode)")
		return true
	}
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		Exitf("Failed to prompt user: %v\n", err)
	}

	ansNormal := strings.ToLower(strings.TrimSpace(answer))
	if len(ansNormal) > 0 && ansNormal[0] == 'n' {
		return false
	}
	return true
}
