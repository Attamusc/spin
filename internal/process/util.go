package process

import "time"

// sleepMs sleeps for the given number of milliseconds.
func sleepMs(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
