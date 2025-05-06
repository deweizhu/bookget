package queue

import (
	"log"
	"sync"
)

// ConcurrentQueue 提供带有限制的并发执行队列
type ConcurrentQueue struct {
	capacity int            // 最大并发数
	sem      chan struct{}  // 信号量通道，用于控制并发
	wg       sync.WaitGroup // 等待所有任务完成
}

// NewConcurrentQueue 创建一个新的并发队列
// capacity: 最大并发数量，必须大于0
func NewConcurrentQueue(capacity int) *ConcurrentQueue {
	if capacity <= 0 {
		panic("queue capacity must be greater than 0")
	}
	return &ConcurrentQueue{
		capacity: capacity,
		sem:      make(chan struct{}, capacity),
	}
}

// Go 提交一个任务到队列中异步执行
// 如果队列已满，会阻塞直到有可用槽位
func (q *ConcurrentQueue) Go(task func()) {
	q.wg.Add(1)
	go func() {
		q.sem <- struct{}{} // 获取信号量
		defer func() {
			<-q.sem // 释放信号量
			q.wg.Done()
		}()

		// 执行任务并处理可能的panic
		defer func() {
			if r := recover(); r != nil {
				log.Printf("task panic recovered: %v", r)
			}
		}()

		task()
	}()
}

// Wait 等待所有已提交的任务完成
func (q *ConcurrentQueue) Wait() {
	q.wg.Wait()
}

// TryGo 尝试提交任务，如果队列已满则立即返回false
func (q *ConcurrentQueue) TryGo(task func()) bool {
	select {
	case q.sem <- struct{}{}: // 尝试获取信号量
		q.wg.Add(1)
		go func() {
			defer func() {
				<-q.sem
				q.wg.Done()
				if r := recover(); r != nil {
					log.Printf("task panic recovered: %v", r)
				}
			}()
			task()
		}()
		return true
	default:
		return false
	}
}

// CurrentCount 返回当前正在执行的任务数量
func (q *ConcurrentQueue) CurrentCount() int {
	return len(q.sem)
}
