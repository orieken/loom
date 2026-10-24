// Package calc is the end-to-end harness's fixture: a library small enough
// that any delivered change to it can be judged by a handful of tests.
package calc

// Add returns the sum of a and b.
func Add(a, b int) int {
	return a + b
}
