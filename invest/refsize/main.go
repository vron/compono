// Command refsize checks what the sise of a compressed ref list would be
package main

import "fmt"

import "bytes"

func main() {
	a := "a:sha2-12980a6c4"
	b := "d:sha2-12980a6c4"

	_ = a
	fmt.Println(bytes.Compare([]byte(a), []byte(b)))
}
