package handler

import (
	"fmt"
	"net/http"
	"strconv"
)

func Fibonacci(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nStr := r.URL.Query().Get("n")
	n := 40
	if i, err := strconv.Atoi(nStr); err == nil && i > 0 {
		n = i
	}

	result := fib(n)
	fmt.Fprintf(w, "%d\n", result)
}

func fib(n int) int {
	if n <= 1 {
		return n
	}
	return fib(n-1) + fib(n-2)
}
