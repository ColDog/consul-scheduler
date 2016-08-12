package main

import (
	"time"
	"fmt"
)

func main()  {
	stop := make(chan struct{})

	go func(stop <-chan struct{}) {
		select {
		case <-stop:
			fmt.Println("exiting")
			return
		}
	}(stop)

	go func(stop <-chan struct{}) {

		for {
			time.Sleep(1 * time.Second)
			select {
			case <-stop:
				fmt.Println("exiting")
				return
			default:
			}
		}

	}(stop)


	time.Sleep(3 * time.Second)
	close(stop)

	time.Sleep(1 * time.Second)

}
