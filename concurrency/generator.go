package concurrency

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

func Count(n int) (chan int) {
	ch := make(chan int)

	go func () {
		for i := 0; i < n; i++ {
			ch <- i
		}
		close(ch)
	}()

	return ch

	//func main() {
	//	c := concurrency.Count(10)
	//	i := <- c
	//	j := <- c
	//	fmt.Println(i)
	//	fmt.Println(j)
	//}
}


func Count2() {
	chanOwner := func() <- chan int {

		// instantiate channel within lexical scope
		results := make(chan int, 5)

		go func() {
			defer close(results)
			for i := 0; i <= 5; i++ {
				results <- i
				time.Sleep(time.Second)
			}
		}()

		// expose read only of channel
		return results
	}

	consumer := func(results <-chan int) {
		time.Sleep(time.Second * 3)
		for result := range results {
			fmt.Printf("Received: %d\n", result)
		}
		fmt.Println("Done receiving")
	}


	results := chanOwner()
	// consumer only gets a read only version of the channels
	consumer(results)
}
// If a goroutine is responsible for creating a goroutine, it is also responsible for ensuring it can stop the goroutine
func PreventingGoRoutineLeaks() {
	doSomeWork := func(done <- chan bool, workToBeDone <- chan string) <- chan bool {
		terminated := make(chan bool)
		go func() {
			defer fmt.Println("doSomeWork is done")
			defer close(terminated)

			for  {
				select {
					case s := <-workToBeDone:
						fmt.Printf("Got %s", s)
					case <- done:
						return
				}
			}
		}()
		return terminated
	}

	done := make(chan bool)

	// Pass nil for workToBeDone!
	terminated := doSomeWork(done, nil)

	go func(){
		time.Sleep(1 * time.Second)
		fmt.Println("Canceling doSomeWork goroutine")

		done <- true
		close(done)
	}()

	// join the goroutine spawned from doSomeWork with the main goRoutine
	<-terminated
	fmt.Println("done")
}


func PreventingGoRoutineLeaks2() {

	randomStream := func(done <-chan interface{}) <-chan int {
		randStream := make(chan int)

		go func() {
			defer close(randStream)

			for {
				select {
					case randStream <- rand.Int():
					case <- done:
						return
				}
			}
		}()
		return randStream
	}

	done := make(chan interface{})
	s := randomStream(done)

	fmt.Println("3 random integers")
	for i := 1; i <= 3; i++ {
		fmt.Printf("%d:  %d\n", i, <-s)
	}
	close(done)
}

// Handling errors:
type Result struct {
	Error error
	Response *http.Response
}

// Errors are first class citizens
func HandleErrorGracefully() {

	checkStatus := func(done <-chan interface{}, urls ...string) <-chan Result {
		results := make(chan Result)

		go func() {
			defer close(results)

			for _, url := range urls {
				var result Result
				resp, err := http.Get(url)
				result = Result{Error: err, Response: resp}

				select {
				case <-done:
					return
				case results <- result:
				}
			}
		}()

		return results
	}

	done := make(chan interface{})
	defer close(done)

	urls := []string{"https://www.fallenstedt.com", "https://www.github.com/Fallenstedt"}
	for result := range checkStatus(done, urls...) {
		if result.Error != nil {
			fmt.Printf("error: %v", result.Error)
			continue
		}
		fmt.Printf("Response: %v\n", result.Response.Status)
	}
}

// Pipelines

func PipelineExample() {
	generator := func(done <- chan interface{}, integers ...int) <-chan int {
		intStream := make(chan int)
		go func() {
			defer close(intStream)
			for _,i := range integers {
				select {
					case <-done:
						return
					case intStream <- i:
				}
			}
		}()

		return intStream
	}

	multiply := func(
		done <-chan interface{},
		intStream <-chan int,
		multiplier int,
	) <-chan int {
		multipliedStream := make(chan int)
		go func() {
			defer close(multipliedStream)
			for i := range intStream {
				select {
					case <-done:
						return
					case multipliedStream <- i*2:
				}
			}
		}()
		return multipliedStream
	}


	add := func(
		done <-chan interface{},
		intStream <-chan int,
		additive int,
	) <-chan int {
		addedStream := make(chan int)
		go func() {
			defer close(addedStream)
			for i := range intStream {
				select {
				case <-done:
					return
				case addedStream <- i + additive:
				}
			}
		}()
		return addedStream
	}

	done := make(chan interface{})
	defer close(done)

	intStream := generator(done, 1, 2, 3, 4)
	pipeline := multiply(done, add(done, multiply(done, intStream, 2), 2), 4)

	for v := range pipeline {
		fmt.Println(v)
	}
}

// Context package example

func ContextExample() {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := printGreeting(ctx); err != nil {
			fmt.Printf("cannot print greenting: %v\n", err)
			cancel()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := printFarewell(ctx); err != nil {
			fmt.Printf("cannot print farewell %v\n", err)
			cancel()
		}
	}()

	wg.Wait()

}
func printGreeting(ctx context.Context) error {
	greeting, err := genGreeting(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("%s world!", greeting)
	return nil
}

func printFarewell(ctx context.Context) error {
	farewell, err := genFarewell(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("%s world!", farewell)
	return nil
}

func genGreeting(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	switch locale, err := locale(ctx); {
	case err != nil:
		return "", err
	case locale == "EN/US":
		return "hello", nil
	}
	return "", fmt.Errorf("unsupported locale")
}

func genFarewell(ctx context.Context) (string, error) {
	switch locale, err := locale(ctx); {
	case err != nil:
		return "", err
	case locale == "EN/US":
		return "goodbye", nil
	}
	return "", fmt.Errorf("unsupported locale")
}

func locale(ctx context.Context) (string, error) {
	select {

		// uncommenting this propagates error up through context
		//case <- ctx.Done():
		//	return "", ctx.Err()
		case <- time.After(1 * time.Second):
	}
	return "EN/US", nil
}