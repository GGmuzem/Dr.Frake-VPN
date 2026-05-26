package main
import "fmt"
import "time"
func main() {
	hours := 24.5
	fmt.Println(time.Duration(hours) * time.Hour)
	fmt.Println(time.Duration(hours * float64(time.Hour)))
}
