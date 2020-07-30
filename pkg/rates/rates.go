package rates

import (
	"time"

        logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("rates")

type Timer interface {
	Run(chan int)
}

type ConstTimer struct {
	Interval int
}

func (t ConstTimer) Run(pin chan int) {
	count := 0
	for {
		select {
		case <- pin:
			log.Info("Exiting Run")
			return
		default:
			if count == t.Interval {
				count = 0
				pin <- 1
			}
			time.Sleep(time.Second)
			count += 1
		}
	}
}

type Rate struct {
	Consumer chan int
	ControlChannel chan bool
}

func (r Rate) SingleRate(t Timer) {
	pin := make(chan int)
	go t.Run(pin)
	for {
		select {
		case <- r.ControlChannel:
			pin <- 1
			log.Info("Exiting SingleRate")
			return
		case n := <- pin:
			select {
			case r.Consumer <- n:
			default:
				log.Info("Could not send to consumer")
			}
		}
	}
}

func (r Rate) IncDecRate(incT Timer, decT Timer, min int, max int) {
	pin := make(chan int)
	goingUp := true
	counter := min
	go incT.Run(pin)
	for {
		select {
		case <- r.ControlChannel:
			pin <- 1
			log.Info("Exiting IncDecRate")
			return
		case <- pin:	// enter this case whenever timer ticks
			if goingUp {
				counter += 1
				if counter == max {
					goingUp = false
					pin <- 1		// stop inc timer
					go decT.Run(pin)	// start dec timer
				}
			} else {
				counter -= 1
				if counter == min {
					goingUp = true
					pin <- 1		// stop dec timer
					go incT.Run(pin)	// start inc timer
				}
			}
			select {
			case r.Consumer <- counter:
			default:
				log.Info("Could not sent to consumer")
			}
		}
	}
}
