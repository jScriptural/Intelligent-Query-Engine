package main

import (
	"fmt"
	"intelliqe/store"
)

func main(){
	fmt.Println("Hello World")

	dbh := store.NewDBHandler("intelliqe.db");

	_ = dbh;
}
