package main

import (
	"fmt"

	"github.com/dave/diskfs"
)

func main() {

	d, err := diskfs.Open("/Users/dave/Desktop/GPT Hiking Tracks (20191023).img")
	if err != nil {
		panic(err)
	}
	fmt.Println(d.Writable)
	fs, err := d.GetFilesystem(0)
	if err != nil {
		panic(err)
	}
	fs.ReadDir("/")

}
