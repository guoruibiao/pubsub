package pubsub

import (
	"sync"
	"fmt"
	"sync/atomic"
	"net"
	"bufio"
)

type Channel struct {
	Name string
	clients map[int]*Client
	sync.RWMutex
	waitGroup sync.WaitGroup
	messageCount uint64
	exitFlag int32
}

func NewChannel(channelName string) *Channel {
	return &Channel{
		Name:channelName,
		clients:make(map[int]*Client),
	}
}

func (this *Channel) AddClient(client *Client) bool {
	this.RLock()
	_, found := this.clients[client.Id]
	this.RUnlock()

	this.Lock()
	if !found {
		this.clients[client.Id] = client
	}
	this.Unlock()
	return found
}

func (this *Channel) DeleteClient(client *Client) int{
	var remain int
	// 整理输出信息
	this.Lock()
	delete(this.clients, client.Id)
	this.Unlock()

	this.RLock()
	remain = len(this.clients)
	this.RUnlock()

	return remain
}

func (this *Channel) Notify(message string) bool {
	this.RLock()
	defer this.RUnlock()

	for clientid, client := range this.clients {
		this.PutMessage(clientid, message)
		go handleResponse(client.conn, message)
	}
	this.waitGroup.Done()
	return true
}

func handleResponse(conn net.Conn, message string) {
	// todo alive check
	fmt.Println("handleResponse: ", message)
	writter := bufio.NewWriter(conn)
	writter.WriteString("[CONSUME MESSAGE] " + message)
	writter.Flush()

}

func (this *Channel) Wait() {
	//this.waitGroup.Wait()
}
func (this *Channel) Exit() {
	if !atomic.CompareAndSwapInt32(&this.exitFlag, 0, 1) {
		return
	}
	this.Wait()
}

func (this *Channel) Exiting() bool {
	return atomic.LoadInt32(&this.exitFlag) == 1
}

func (this *Channel) PutMessage(clientid int, message string) {
	this.RLock()
	defer this.RUnlock()

	if this.Exiting() {
		return
	}
	fmt.Println("{", this.Name, "}[", clientid, "] ", message)
	atomic.AddUint64(&this.messageCount, 1)
	return
}