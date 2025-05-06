package app

type QueueLimit struct {
	n int
	c chan interface{}
}

func QueueNew(n int) *QueueLimit {
	return &QueueLimit{
		n: n,
		c: make(chan interface{}, n),
	}
}
func (q *QueueLimit) Go(f func()) {
	q.c <- 0
	go func() {
		f()
		<-q.c
	}()
}
