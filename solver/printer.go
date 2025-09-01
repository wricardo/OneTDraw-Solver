package solver

import (
	"encoding/json"
	"fmt"
)

type SolutionPrinter interface {
	Print(solutions *Solutions)
}

type JsonPrinter struct{}

func (this JsonPrinter) Print(s *Solutions) {
	a, _ := json.Marshal(s)
	fmt.Println(string(a))
}

type CleanPrinter struct{}

func (this CleanPrinter) Print(s *Solutions) {
	for _, v := range *s {
		l := len(v)
		for k, v1 := range v {
			if k < l-1 {
				fmt.Printf("%v - ", v1)
			} else {
				fmt.Printf("%v", v1)
			}
		}
		fmt.Println("")
	}
}
