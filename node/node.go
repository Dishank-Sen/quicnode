package node

import "fmt"

type Node struct{

}

func NewNode(){
	fmt.Println("hello from quicp2p")
}

func Greet(name string) string{
	return fmt.Sprintf("hello %s", name)
}