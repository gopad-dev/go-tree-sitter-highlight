// Package main is the entry point of the program.
package main

import "fmt"

// msg1 is a message
const msg1 = "Hello,"

// msg2 is another message
const msg2 = "World!"

// foo is a struct
type foo struct {
	// baz is a string
	baz string
}

// bar is a method of foo
func (f *foo) bar() {}

// bar2 is a method of foo
func (f foo) bar2() {}

// TODO: Add more comments
func main() {
	fmt.Println(msg1, msg2)
}
