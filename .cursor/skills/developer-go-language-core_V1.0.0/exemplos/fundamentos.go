// Package main demonstra os fundamentos cobertos por developer-go-language-core:
// variáveis, control flow, funções com múltiplos retornos e funções variádicas.
package main

import "fmt"

// dividir retorna o quociente e um error explícito quando o divisor é zero.
// Convenção idiomática: o error é sempre o último valor de retorno.
func dividir(a, b float64) (float64, error) {
	if b == 0 {
		return 0, fmt.Errorf("divisão por zero: %v / %v", a, b)
	}
	return a / b, nil
}

// soma é uma função variádica: aceita zero ou mais argumentos int.
func soma(nums ...int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}

// classificarIdade demonstra switch sem fallthrough implícito.
func classificarIdade(idade int) string {
	switch {
	case idade < 12:
		return "criança"
	case idade < 18:
		return "adolescente"
	default:
		return "adulto"
	}
}

func main() {
	// var com tipo explícito, var com zero value, := com inferência.
	var nome string = "Go"
	var contador int
	idade := 30

	fmt.Printf("linguagem=%s contador=%d idade=%d\n", nome, contador, idade)

	// if / for
	if idade >= 18 {
		fmt.Println("maior de idade")
	} else {
		fmt.Println("menor de idade")
	}

	for i := 0; i < 3; i++ {
		fmt.Println("iteração", i)
	}

	// switch
	fmt.Println("classificação:", classificarIdade(idade))

	// função com múltiplos retornos — tratamento explícito de erro
	resultado, err := dividir(10, 0)
	if err != nil {
		fmt.Println("erro esperado:", err)
	} else {
		fmt.Println("resultado:", resultado)
	}

	// função variádica
	fmt.Println("soma:", soma(1, 2, 3, 4, 5))
}
